package server

import (
	"GijzaFiler/rsacrypto"
	"GijzaFiler/utils"
	"bufio"
	"bytes"
	"crypto/rsa"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Default port of server
const DEFAULTPORT int = 5416

// Requires entering the data of server from user
func CollectServerData() (int, string, bool, []string, int) {
	ml := utils.Logger{Prefix: ""}
	errl := utils.Logger{Prefix: "Error"}
	var Port int           // Port of server
	var Dirname string     // Work dir of server
	var Passwords []string // Passwords for access of server
	for {                  // Requires enter server port
		sport := ml.Input("Enter port (default " + fmt.Sprint(DEFAULTPORT) + "): ")
		if sport == "" {
			Port = DEFAULTPORT
			break
		}
		iport, err := strconv.Atoi(sport)
		if err != nil {
			errl.PPrintln("Port must be numeric!")
			continue
		}
		if iport < 22 || iport > 65353 {
			errl.PPrintln("The port must be in the range: 22-65353")
			continue
		}
		Port = iport
		break
	} // Requires enter work dir
	for {
		sdirname := ml.Input("Enter directory path: ")
		if utils.ExistsDirOrFile(false, true, sdirname) {
			Dirname = sdirname
			break
		} else {
			errl.PPrintln("Directory not found!")
			continue
		}
	}
	Encryption := strings.ToLower(ml.Input("Protect the connection with end-to-end encryption? [Y/n] ")) == "y"
	for { // Requires enter passwords
		password := ml.Input("Enter password #" + fmt.Sprint(len(Passwords)+1) + ": ")
		if password == "" {
			break
		} else {
			Passwords = append(Passwords, password)
		}
	}
	ConnectionLimit := -1
	mkconlim := strings.ToLower(ml.Input("Set a limit on the number of connected users? [Y/n] ")) == "y"
	if mkconlim {
		for {
			sconlim := ml.Input("What is the maximum number of users that will be connected at the same time? ")
			iconlim, err := strconv.Atoi(sconlim)
			if err != nil {
				errl.PPrintln("Connection limit must be numeric!")
				continue
			}
			if iconlim < 1 {
				errl.PPrintln("Connection limit must be greater than 1")
				continue
			}
			ConnectionLimit = iconlim
			break
		}
	}
	return Port, Dirname, Encryption, Passwords, ConnectionLimit
}

type Server struct {
	Port             int
	Directory        string
	Passwords        []string
	BytesLimit       int
	ConnectionsLimit int
	ConnectionCount  int
	Encryption       bool
	listener         net.Listener
}

// Create server instance with own data
func Create(port int, directory string, encrypt bool, passwords []string, connectionLimit int) Server {
	return Server{Port: port, Directory: directory, Passwords: passwords, BytesLimit: 2048, ConnectionsLimit: connectionLimit, ConnectionCount: 0, Encryption: encrypt}
}

// Run server listening
func (this *Server) Run() {
	inf := utils.Logger{Prefix: "server"}
	errl := utils.Logger{Prefix: "error"}
	inf.PPrintln("Server starting on port " + fmt.Sprint(this.Port))
	listen, err := net.Listen("tcp", ":"+fmt.Sprint(this.Port))
	if err != nil {
		errl.PPrintln("An error occurred while creating the server: " + err.Error())
		port, directory, encryption, passwords, connectionLimit := CollectServerData()
		this.Port = port
		this.Directory = directory
		this.Encryption = encryption
		this.Passwords = passwords
		this.ConnectionsLimit = connectionLimit
		this.Run()
		return
	}
	this.listener = listen
	inf.PPrintln("Started, waiting for connection...")
	for {
		con, err := listen.Accept()
		if err != nil {
			con.Close()
			errl.PPrintln("An error occurred: " + err.Error())
			continue
		}
		// Checking client count limit
		if this.ConnectionCount+1 > this.ConnectionsLimit && this.ConnectionsLimit != -1 {
			con.Close()
		} else {
			this.ConnectionCount++
			go this.ClientHandler(con) // Creating new thread for working with client
		}
	}
}

// Function for defer, decrements count of clients
func (this *Server) MinusConnection() {
	this.ConnectionCount--
}

// Function for working with clients
func (this Server) ClientHandler(con net.Conn) {
	defer this.MinusConnection()
	inf := utils.Logger{Prefix: "server"}
	errl := utils.Logger{Prefix: "error"}
	inf.PPrintln(con.RemoteAddr().String() + " connected!")

	// Close connection with client on return
	defer con.Close()
	defer inf.PPrintln(con.RemoteAddr().String() + " disconnected!")

	// Time limit to send first message
	con.SetDeadline(time.Now().Add(2 * time.Second))

	// Do client entered password
	var authed bool = false
	var publKey *rsa.PublicKey = nil
	var privKey *rsa.PrivateKey = nil

	// Listening him messages
	for {
		// Reading message from client
		req, err := this.ReadMessage(con, privKey)
		if err != nil {
			errl.PPrintln("Receiving message error: " + err.Error())
			return
		}

		// Handling messages by him auth status
		if !authed {
			// When client is not authed
			disconnect, doAuthed := this.NotAuthedHandler(con, req, &publKey, &privKey)
			if disconnect {
				return
			}
			if doAuthed {
				authed = true
			}
		} else {
			// When client is authed
			diconnect := this.AuthedHandler(con, req, publKey, privKey)
			if diconnect {
				return
			}
		}
	}
}

// Handler of not authed client
func (this *Server) NotAuthedHandler(con net.Conn, req []interface{}, publKey **rsa.PublicKey, privKey **rsa.PrivateKey) (bool, bool) { // 1st bool - close connection, 2d bool - change status to authed
	inf := utils.Logger{Prefix: "server"}
	errl := utils.Logger{Prefix: "error"}

	if len(req) == 0 { // Prevalidation
		return true, false
	}

	if req[0] == "connect" { // Client want to connect
		con.SetDeadline(time.Time{}) // Cleaning time limit

		// Set sucure connection if Encryption field is true
		if this.Encryption {
			if *publKey == nil {
				var publKeyToSend *rsa.PublicKey // We want send this key to client
				*privKey, publKeyToSend, _ = rsacrypto.GenerateKeyPair(rsacrypto.KeySize)
				publKeyToSendInString, _ := rsacrypto.PublicKeyToBytes(publKeyToSend)

				res, _ := this.ListToMessage([]interface{}{"firstPublicKey", publKeyToSendInString}, *publKey)
				_, err := con.Write(res)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true, false
				}

				return false, false
			} else {
				inf.PPrintln(con.RemoteAddr().String() + " connection encrypted!")
			}
		}

		if len(this.Passwords) == 0 {
			// When server have no passwords
			res, _ := this.ListToMessage([]interface{}{"success"}, *publKey)
			_, err := con.Write(res)
			if err != nil {
				errl.PPrintln("Sending error: " + err.Error())
				return true, false
			}
			return false, true
		} else {
			// Validating passwords
			res, _ := this.ListToMessage([]interface{}{"enter_password", len(this.Passwords)}, *publKey)
			_, err := con.Write(res)
			if err != nil {
				errl.PPrintln("Sending error: " + err.Error())
				return true, false
			}
		}
	} else if req[0] == "publicKey" && len(req) == 2 {
		if key, ok := req[1].([]byte); ok {
			var err error
			*publKey, err = rsacrypto.BytesToPublicKey(key)
			if err != nil {
				return true, false
			}

			// Renew public key
			var publKeyToSend *rsa.PublicKey // We want send this key to client
			*privKey, publKeyToSend, _ = rsacrypto.GenerateKeyPair(rsacrypto.KeySize)
			publKeyToSendInString, _ := rsacrypto.PublicKeyToBytes(publKeyToSend)

			res, _ := this.ListToMessage([]interface{}{"secondPublicKey", publKeyToSendInString}, *publKey)
			_, err = con.Write(res)
			if err != nil {
				errl.PPrintln("Sending error: " + err.Error())
				return true, false
			}

			return false, false
		} else {
			return true, false
		}
	} else if req[0] == "password" && len(this.Passwords) != 0 { // Client want to get access entering passwords
		if len(req)-1 != len(this.Passwords) {
			res, _ := this.ListToMessage([]interface{}{"fail"}, *publKey)
			_, err := con.Write(res)
			if err != nil {
				errl.PPrintln("Sending error: " + err.Error())
				return true, false
			}
		} else {
			var success bool = true
			for i, p := range this.Passwords {
				if req[i+1] != p {
					success = false
					break
				}
			}
			if success {
				res, _ := this.ListToMessage([]interface{}{"success"}, *publKey)
				_, err := con.Write(res)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true, false
				}
				inf.PPrintln(con.RemoteAddr().String() + " signed in!")
				return false, true
			} else {
				res, _ := this.ListToMessage([]interface{}{"fail"}, *publKey)
				_, err := con.Write(res)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true, false
				}
			}
		}
	} else { // Disconnecting client when he sending invalid command
		errl.PPrintln("Client sent unknown command")
		return true, false
	}
	return false, false
}

// Handler of authed client
func (this *Server) AuthedHandler(con net.Conn, req []interface{}, publKey *rsa.PublicKey, privKey *rsa.PrivateKey) bool { // bool - close connection
	errl := utils.Logger{Prefix: "error"}
	if req[0] == "get_folders" && len(req) == 2 { // Client want to get folder list
		if foldname, ok := req[1].(string); ok {
			var splitted []string = strings.Split(strings.ReplaceAll(foldname, "\\", "/"), "/")
			var success bool = true
			if foldname != "" {
				for i, a := range splitted { // Folder path out protection
					stat, err := os.ReadDir(path.Join(this.Directory, strings.Join(splitted[:i], "/")))
					if err != nil {
						res, _ := this.ListToMessage([]interface{}{"fail", err.Error()}, publKey)
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return true
						}
						success = false
						break
					}
					var ss bool = false
					for _, nm := range stat {
						if nm.IsDir() {
							if nm.Name() == a {
								ss = true
								break
							}
						}
					}
					if ss {
						continue
					} else {
						success = false
						break
					}
				}
			}
			if !success {
				res := []interface{}{"fail", "folder not found!"}
				re, _ := this.ListToMessage(res, publKey)
				_, err := con.Write(re)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
			res := []interface{}{"success"}
			stat, err := os.ReadDir(path.Join(this.Directory, foldname))
			if err != nil {
				res, _ := this.ListToMessage([]interface{}{"fail", err.Error()}, publKey)
				_, err = con.Write(res)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
			for _, nm := range stat {
				if nm.IsDir() {
					res = append(res, nm.Name())
				}
			}
			re, _ := this.ListToMessage(res, publKey)
			_, err = con.Write(re)
			if err != nil {
				errl.PPrintln("Sending error: " + err.Error())
				return true
			}
		} else {
			errl.PPrintln("Client sent unknown command")
			return true
		}
	} else if req[0] == "get_files" && len(req) == 2 { // Client want to get file list
		if foldname, ok := req[1].(string); ok {
			var splitted []string = strings.Split(strings.ReplaceAll(foldname, "\\", "/"), "/")
			var success bool = true
			if foldname != "" {
				for i, a := range splitted {
					stat, err := os.ReadDir(path.Join(this.Directory, strings.Join(splitted[:i], "/")))
					if err != nil {
						res, _ := this.ListToMessage([]interface{}{"fail", err.Error()}, publKey)
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return true
						}
						success = false
						break
					}
					var ss bool = false
					for _, nm := range stat {
						if nm.IsDir() {
							if nm.Name() == a {
								ss = true
								break
							}
						}
					}
					if ss {
						continue
					} else {
						success = false
						break
					}
				}
			}
			if !success {
				res := []interface{}{"fail", "folder not found!"}
				re, _ := this.ListToMessage(res, publKey)
				_, err := con.Write(re)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
			res := []interface{}{"success"}
			stat, err := os.ReadDir(path.Join(this.Directory, foldname))
			if err != nil {
				res, _ := this.ListToMessage([]interface{}{"fail", err.Error()}, publKey)
				_, err = con.Write(res)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
			for _, nm := range stat {
				if !nm.IsDir() {
					res = append(res, nm.Name())
				}
			}
			re, _ := this.ListToMessage(res, publKey)
			_, err = con.Write(re)
			if err != nil {
				errl.PPrintln("Sending error: " + err.Error())
				return true
			}
		} else {
			errl.PPrintln("Client sent unknown command")
			return true
		}
	} else if req[0] == "download" && len(req) == 2 { // Getting file content or directory tree
		if foldname, ok := req[1].(string); ok {
			if foldname == "." {
				res := []interface{}{"success"}
				dirls := []string{}
				fils := []string{}
				IterFolder(this.Directory, "", &dirls, &fils)
				res = append(res, dirls)
				res = append(res, fils)
				re, _ := this.ListToMessage(res, publKey)
				_, err := con.Write(re)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
			foldname = strings.ReplaceAll(foldname, "\\", "/")
			var splitted []string = strings.Split(strings.ReplaceAll(foldname, "\\", "/"), "/")
			var success bool = true
			if foldname != "" {
				for i, a := range splitted[:len(splitted)-1] {
					stat, err := os.ReadDir(path.Join(this.Directory, strings.Join(splitted[:i], "/")))
					if err != nil {
						res, _ := this.ListToMessage([]interface{}{"fail", err.Error()}, publKey)
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return true
						}
						success = false
						break
					}
					var ss bool = false
					for _, nm := range stat {
						if nm.IsDir() {
							if nm.Name() == a {
								ss = true
								break
							}
						}
					}
					if ss {
						continue
					} else {
						success = false
						break
					}
				}
			}
			if !success {
				res := []interface{}{"fail", "folder not found!"}
				re, _ := this.ListToMessage(res, publKey)
				_, err := con.Write(re)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
			res := []interface{}{"success"}
			stat, err := os.ReadDir(path.Dir(path.Join(this.Directory, foldname)))
			if err != nil {
				res, _ := this.ListToMessage([]interface{}{"fail", err.Error()}, publKey)
				_, err = con.Write(res)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
			var asuccess bool = true
			var finded bool = false
			for _, nm := range stat {
				if nm.Name() == splitted[len(splitted)-1] {
					finded = true
					if nm.IsDir() {
						res = append(res, "folder")
						var dirls []string
						var fils []string
						IterFolder(path.Join(this.Directory, foldname), splitted[len(splitted)-1], &dirls, &fils)
						res = append(res, dirls)
						res = append(res, fils)
					} else {
						res = append(res, "file")
						cont, err := os.ReadFile(path.Join(this.Directory, foldname))
						if err != nil {
							res := []interface{}{"fail", "the file cannot be read"}
							re, _ := this.ListToMessage(res, publKey)
							_, err = con.Write(re)
							if err != nil {
								errl.PPrintln("Sending error: " + err.Error())
								return true
							}
							asuccess = false
							break
						}
						res = append(res, cont)
					}
					break
				}
			}
			if !asuccess {
				return false
			}
			if finded {
				re, _ := this.ListToMessage(res, publKey)
				_, err = con.Write(re)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
			} else {
				res := []interface{}{"fail", "folder/file not found!"}
				re, _ := this.ListToMessage(res, publKey)
				_, err = con.Write(re)
				if err != nil {
					errl.PPrintln("Sending error: " + err.Error())
					return true
				}
				return false
			}
		} else {
			errl.PPrintln("Client sent unknown command")
			return true
		}
	} else {
		errl.PPrintln("Client sent unknown command")
		return true
	}
	return false
}

// Get directory file tree using recursion
func IterFolder(path string, write_as string, dirls *[]string, fils *[]string) {
	p, err := os.ReadDir(path)
	if err != nil {
		return
	}

	// Looping folder content
	for _, u := range p {
		if u.IsDir() {
			*dirls = append(*dirls, filepath.Join(write_as, u.Name()))
			IterFolder(filepath.Join(path, u.Name()), filepath.Join(write_as, u.Name()), dirls, fils)
		} else {
			*fils = append(*fils, filepath.Join(write_as, u.Name()))
		}
	}
}

// Receiving message from client
func (this Server) ReadMessage(client net.Conn, privKey *rsa.PrivateKey) ([]interface{}, error) {
	reader := bufio.NewReader(client)
	message := make([]byte, 0)

	for {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if err != nil {
			return []interface{}{}, err
		}
		message = append(message, buf[:n]...)
		if len(message) > this.BytesLimit {
			return []interface{}{}, fmt.Errorf("bytes limit")
		}
		if n < 1024 {
			break
		}
		if privKey != nil {
			_, err := rsacrypto.DecryptWithPrivateKey(message, privKey)
			if err != nil {
				continue
			}
		}
		var ls []interface{}
		var buffer bytes.Buffer
		buffer.Write(message)
		decoder := gob.NewDecoder(&buffer)
		err = decoder.Decode(&ls)
		if err != nil {
			continue
		} else {
			break
		}
	}

	if privKey != nil {
		msg, err := rsacrypto.DecryptWithPrivateKey(message, privKey)
		if err != nil {
			return []interface{}{}, err
		}
		message = msg
	}

	var ret []interface{}
	var buffer bytes.Buffer
	buffer.Write(message)
	decoder := gob.NewDecoder(&buffer)
	err := decoder.Decode(&ret)
	if err != nil {
		return []interface{}{}, err
	}

	return ret, nil
}

// Converting data to bytes for sending
func (this Server) ListToMessage(list []interface{}, publKey *rsa.PublicKey) ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(list)
	if err != nil {
		return []byte{}, err
	}
	ret := buff.Bytes()
	if publKey != nil {
		enc, err := rsacrypto.EncryptWithPublicKey(ret, publKey)
		if err != nil {
			return []byte{}, err
		}
		return enc, nil
	}
	return ret, nil
}
