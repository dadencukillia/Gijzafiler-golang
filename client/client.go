package client

import (
	"GijzaFiler/rsacrypto"
	"GijzaFiler/server"
	"GijzaFiler/utils"
	"bufio"
	"bytes"
	"crypto/rsa"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Default port of server
const DEFAULTPORT int = 5416

// Parse ip and port from user input
func GetPortAndIp(inp string) (string, int) {
	ip_port_splitted := strings.Split(inp, "/")
	ip_port := ip_port_splitted[len(ip_port_splitted)-1]

	splitted := strings.Split(ip_port, ":")
	ip := splitted[0]

	var port int
	if len(splitted) < 2 {
		port = DEFAULTPORT
	} else {
		newport, err := strconv.Atoi(splitted[1])
		if err != nil || newport < 22 || newport > 65353 {
			port = DEFAULTPORT
		} else {
			port = newport
		}
	}
	return ip, port
}

// Requires entering the data of client from user
func CollectClientData() (string, int) {
	ml := utils.Logger{Prefix: ""}

	ip_port := ml.Input("Enter IP address: ")
	return GetPortAndIp(ip_port)
}

type Client struct {
	Ip         string
	Port       int
	PublKey    *rsa.PublicKey
	PrivKey    *rsa.PrivateKey
	connection net.Conn
}

// Create client instance with own data
func Create(ip string, port int) Client {
	return Client{Ip: ip, Port: port, PublKey: nil, PrivKey: nil}
}

// Connect to server
func (this *Client) Run() {
	inf := utils.Logger{Prefix: "client"}
	errl := utils.Logger{Prefix: "error"}
	address := this.Ip + ":" + fmt.Sprint(this.Port)
	inf.PPrintln("Connecting to " + address + "...")
	connection, err := net.Dial("tcp", address)
	this.connection = connection
	// Require enter new data when error connect
	if err != nil {
		errl.PPrintln("Connection error!")
		ip, port := CollectClientData()
		this.Ip = ip
		this.Port = port
		this.Run()
		return
	}
	// Start client session
	this.runSession()
}

func (this *Client) runSession() {
	inf := utils.Logger{Prefix: "client"}
	errl := utils.Logger{Prefix: "error"}
	con := this.connection
	inf.PPrintln("Connected!")
	res, _ := this.ListToMessage([]interface{}{"connect"}) // Start message
	con.Write(res)                                         // Send message
	var count int = 0
	// Authing loop
	for {
		nmsg, err := this.ReadMessage()
		if err != nil {
			errl.PPrintln("An error occurred: " + err.Error())
			con.Close()
			return
		}

		if nmsg[0] == "success" {
			if count == 0 && this.PublKey == nil {
				inf.PPrintln("⚠️ The connection is not protected")
			}
			break
		} else if nmsg[0] == "firstPublicKey" {
			inf.PPrintln("🔒 The connection is protected by E2EE technology")
			if key, ok := nmsg[1].([]byte); ok {
				this.PublKey, err = rsacrypto.BytesToPublicKey(key)

				if err != nil {
					errl.PPrintln("Suspect connection: " + err.Error())
					con.Close()
					return
				}

				// Generating key for server
				var publKeyToSend *rsa.PublicKey // We want send this key to server
				this.PrivKey, publKeyToSend, _ = rsacrypto.GenerateKeyPair(rsacrypto.KeySize)
				publKeyToSendInString, _ := rsacrypto.PublicKeyToBytes(publKeyToSend)

				list := []interface{}{"publicKey", publKeyToSendInString}
				toSend, _ := this.ListToMessage(list)
				con.Write(toSend)
			} else {
				errl.PPrintln("Suspect connection: " + err.Error())
				con.Close()
				return
			}
		} else if nmsg[0] == "secondPublicKey" {
			if key, ok := nmsg[1].([]byte); ok {
				this.PublKey, err = rsacrypto.BytesToPublicKey(key)
				if err != nil {
					errl.PPrintln("Suspect connection: " + err.Error())
					con.Close()
					return
				}

				list := []interface{}{"connect"}
				toSend, _ := this.ListToMessage(list)
				con.Write(toSend)
			} else {
				errl.PPrintln("Suspect connection: " + err.Error())
				con.Close()
				return
			}
		} else if nmsg[0] == "enter_password" {
			if this.PublKey == nil {
				inf.PPrintln("⚠️ The connection is not protected")
			}
			if c, ok := nmsg[1].(int); ok {
				count = int(c)
				inf.PPrintln("The server requires entering " + fmt.Sprint(nmsg[1]) + " passwords for access")
				var passwords []string
				for u := 1; u <= int(c); u++ {
					passwords = append(passwords, inf.Input("Enter password #"+fmt.Sprint(u)+": "))
				}
				list := []interface{}{"password"}
				for _, pass := range passwords {
					list = append(list, pass)
				}
				toSend, _ := this.ListToMessage(list)
				con.Write(toSend)
			}
		} else if nmsg[0] == "fail" {
			errl.PPrintln("Incorrect passwords! Try again")
			var passwords []string
			for u := 1; u <= count; u++ {
				passwords = append(passwords, inf.Input("Enter password #"+fmt.Sprint(u)+": "))
			}
			list := []interface{}{"password"}
			for _, pass := range passwords {
				list = append(list, pass)
			}
			toSend, _ := this.ListToMessage(list)
			con.Write(toSend)
		}
	}
	this.authedSession() // Continue session of authed client
}

func (this *Client) authedSession() {
	con := this.connection
	inf := utils.Logger{Prefix: "client"}
	errl := utils.Logger{Prefix: "error"}
	inf.PPrintln("Signed in successfully!")
	inf.Println("")
	inf.Println("Type \"help\" to get a list of available functions")
	var path []string = []string{"."}
	// Cycle of user commands
	for {
		cmd := inf.Input("/$ ")
		splitted := strings.Split(cmd, " ")
		if splitted[0] == "help" { // Prints functions hint
			inf.Println("• help\n• neofetch\n• ls\n• cd <folder name>\n• pwd\n• wget <folder or file name>\n• cat <file name>\n• disconnect\n• exit")
		} else if splitted[0] == "neofetch" { // prints gijzafiler logo
			inf.DrawLogo()
		} else if splitted[0] == "ls" { // Prints list of files and folders in current folder
			res, _ := this.ListToMessage([]interface{}{"get_folders", strings.Join(path[1:], "/")})
			_, err := con.Write(res)
			if err != nil {
				errl.PPrintln("Error getting information")
				continue
			}
			folders, err := this.ReadMessage()
			if err != nil {
				errl.PPrintln("Error getting information")
				continue
			}
			res, _ = this.ListToMessage([]interface{}{"get_files", strings.Join(path[1:], "/")})
			_, err = con.Write(res)
			if err != nil {
				errl.PPrintln("Error getting information")
				continue
			}
			files, err := this.ReadMessage()
			if err != nil {
				errl.PPrintln("Error getting information")
				continue
			}
			if folders[0] == "success" {
				folders = folders[1:]
			} else {
				if er, ok := folders[1].(string); ok {
					errl.PPrintln("Error getting list of folders: " + er)
				}
				continue
			}
			if files[0] == "success" {
				files = files[1:]
			} else {
				if er, ok := files[1].(string); ok {
					errl.PPrintln("Error getting list of files: " + er)
				}
				continue
			}
			inf.Println("Folders:")
			if len(folders) != 0 {
				for _, a := range folders {
					if nam, ok := a.(string); ok {
						inf.Println("• " + nam)
					}
				}
			} else {
				inf.Println("Nothing here")
			}
			inf.Println("Files:")
			if len(files) != 0 {
				for _, a := range files {
					if nam, ok := a.(string); ok {
						inf.Println("• " + nam)
					}
				}
			} else {
				inf.Println("Nothing here")
			}
		} else if splitted[0] == "cd" && len(splitted) > 1 { // Changes current directory
			name := strings.Join(splitted[1:], " ")
			if name == ".." {
				if len(path) <= 1 {
					errl.PPrintln("You cannot level up in this folder")
					continue
				}
				path = path[:len(path)-1]
			} else if name == "." {
				path = []string{"."}
				inf.Println("Successfully!")
			} else {
				res, _ := this.ListToMessage([]interface{}{"get_folders", strings.Join(path[1:], "/")})
				_, err := con.Write(res)
				if err != nil {
					errl.PPrintln("Error sending request to retrieve folders")
					continue
				}
				folders, err := this.ReadMessage()
				if err != nil {
					errl.PPrintln("Error retrieving folders")
					continue
				}
				if folders[0] != "success" {
					errl.PPrint("No rights")
					if msg, ok := folders[1].(string); ok {
						errl.PPrintln(": " + msg)
					} else {
						errl.PPrintln("")
					}
					continue
				}
				if !sliceContainsValue(folders, name) {
					errl.PPrintln("Folder with name \"" + name + "\" not found!")
					continue
				}
				path = append(path, name)
				inf.Println("Successfully!")
			}
		} else if splitted[0] == "pwd" { // Prints current path
			inf.Println(strings.Join(path, "/"))
		} else if splitted[0] == "wget" && len(splitted) > 1 { // Download file or folder
			file_or_dir_name := strings.Join(splitted[1:], " ")
			if file_or_dir_name != "." {
				file_or_dir_path_splitted := []string{}
				file_or_dir_path_splitted = append(file_or_dir_path_splitted, path[1:]...)
				file_or_dir_path_splitted = append(file_or_dir_path_splitted, file_or_dir_name)
				file_or_dir_path := strings.Join(file_or_dir_path_splitted, "/")
				res, _ := this.ListToMessage([]interface{}{"download", file_or_dir_path})
				_, err := con.Write(res)
				if err != nil {
					errl.PPrintln("Error sending request")
					continue
				}
				resp, err := this.ReadMessage()
				if err != nil {
					errl.PPrintln("Error getting information")
					continue
				}
				if resp[0] != "success" {
					if er, ok := resp[1].(string); ok {
						errl.PPrintln(er)
					}
					continue
				}
				if resp[1] == "file" {
					if bts, ok := resp[2].([]byte); ok {
						err = os.WriteFile(file_or_dir_name, bts, 0644)
						if err != nil {
							errl.PPrintln("File writing error: " + err.Error())
							continue
						}
						f, err := filepath.Abs(file_or_dir_name)
						if err != nil {
							inf.Println("Successfully saved to file!")
						} else {
							inf.Println("Successfully saved to file: " + f)
						}
					}
				} else {
					var dir_count int = 0
					var files_count int = 0
					var dir_skip_count int = 0
					var files_skip_count int = 0
					if dirls, ok := resp[2].([]string); ok {
						for _, u := range dirls {
							dir_count++
							if os.MkdirAll(u, 0644) != nil {
								dir_skip_count++
							}
						}
					}
					if fils, ok := resp[3].([]string); ok {
						for _, u := range fils {
							files_count++
							ufile_or_dir_name := u
							ufile_or_dir_path := filepath.Join(append(path[1:], ufile_or_dir_name)...)
							res, _ := this.ListToMessage([]interface{}{"download", ufile_or_dir_path})
							_, err := con.Write(res)
							if err != nil {
								files_skip_count++
								continue
							}
							resp, err := this.ReadMessage()
							if err != nil {
								files_skip_count++
								continue
							}
							if resp[0] != "success" {
								files_skip_count++
								continue
							}
							if resp[1] == "file" {
								if bts, ok := resp[2].([]byte); ok {
									err = os.WriteFile(ufile_or_dir_name, bts, 0644)
									if err != nil {
										files_skip_count++
										continue
									}
								} else {
									files_skip_count++
								}
							} else {
								files_skip_count++
							}
						}
					}
					a, err := os.Getwd()
					if err != nil {
						inf.Println("Successfully saved to folder!")
					} else {
						inf.Println("Successfully saved to folder: " + filepath.Join(a, file_or_dir_name))
					}
					inf.Println("Folders were downloaded: " + fmt.Sprint(dir_count-dir_skip_count) + "/" + fmt.Sprint(dir_count))
					inf.Println("Files were downloaded: " + fmt.Sprint(files_count-files_skip_count) + "/" + fmt.Sprint(files_count))
				}
			} else {
				res, _ := this.ListToMessage([]interface{}{"download", "."})
				_, err := con.Write(res)
				if err != nil {
					errl.PPrintln("Error sending request")
					continue
				}
				resp, err := this.ReadMessage()
				if err != nil {
					errl.PPrintln("Error getting information")
					continue
				}
				if resp[0] != "success" {
					if er, ok := resp[1].(string); ok {
						errl.PPrintln(er)
					}
					continue
				}
				var file_or_dir_name string = "Session" + uuid.NewString()
				var dir_count int = 0
				var files_count int = 0
				var dir_skip_count int = 0
				var files_skip_count int = 0
				if dirls, ok := resp[1].([]string); ok {
					for _, u := range dirls {
						dir_count++
						if os.MkdirAll(filepath.Join(file_or_dir_name, u), 0644) != nil {
							dir_skip_count++
						}
					}
				}
				if fils, ok := resp[2].([]string); ok {
					for _, u := range fils {
						files_count++
						ufile_or_dir_name := u
						ufile_or_dir_path := filepath.Join(append(path[1:], ufile_or_dir_name)...)
						res, _ := this.ListToMessage([]interface{}{"download", ufile_or_dir_path})
						_, err := con.Write(res)
						if err != nil {
							files_skip_count++
							continue
						}
						resp, err := this.ReadMessage()
						if err != nil {
							files_skip_count++
							continue
						}
						if resp[0] != "success" {
							files_skip_count++
							continue
						}
						if resp[1] == "file" {
							if bts, ok := resp[2].([]byte); ok {
								err = os.WriteFile(filepath.Join(file_or_dir_name, ufile_or_dir_name), bts, 0644)
								if err != nil {
									files_skip_count++
									continue
								}
							} else {
								files_skip_count++
							}
						} else {
							files_skip_count++
						}
					}
				}
				a, err := os.Getwd()
				if err != nil {
					inf.Println("Successfully saved to folder!")
				} else {
					inf.Println("Successfully saved to folder: " + filepath.Join(a, file_or_dir_name))
				}
				inf.Println("Folders were downloaded: " + fmt.Sprint(dir_count-dir_skip_count) + "/" + fmt.Sprint(dir_count))
				inf.Println("Files were downloaded: " + fmt.Sprint(files_count-files_skip_count) + "/" + fmt.Sprint(files_count))
			}
		} else if splitted[0] == "cat" && len(splitted) > 1 { // Prints content of file
			file_or_dir_name := strings.Join(splitted[1:], " ")
			file_or_dir_path_splitted := []string{}
			file_or_dir_path_splitted = append(file_or_dir_path_splitted, path[1:]...)
			file_or_dir_path_splitted = append(file_or_dir_path_splitted, file_or_dir_name)
			file_or_dir_path := strings.Join(file_or_dir_path_splitted, "/")
			res, _ := this.ListToMessage([]interface{}{"download", file_or_dir_path})
			_, err := con.Write(res)
			if err != nil {
				errl.PPrintln("Error sending request")
				continue
			}
			resp, err := this.ReadMessage()
			if err != nil {
				errl.PPrintln("Error getting information")
				continue
			}
			if resp[0] != "success" {
				if er, ok := resp[1].(string); ok {
					errl.PPrintln(er)
				}
				continue
			}
			if resp[1] == "file" {
				if bts, ok := resp[2].([]byte); ok {
					inf.Println(string(bts))
				}
			} else {
				errl.PPrintln(file_or_dir_name + " is not a file!")
				continue
			}
		} else if splitted[0] == "disconnect" { // Disconnects from server
			con.Close()
			utils.ClearTerminal()
			StarterMenu()
			return
		} else if splitted[0] == "exit" { // Disconnects and exits
			con.Close()
			return
		} else { // Not listened
			errl.PPrintln("Unknown command")
		}
	}
}

// Check if var a contains in slice
func sliceContainsValue(slice []any, a any) bool {
	for _, u := range slice {
		if u == a {
			return true
		}
	}
	return false
}

// Set private key
func (this *Client) SetPrivateKey(pk *rsa.PrivateKey) {
	this.PrivKey = pk
}

// Set public key
func (this *Client) SetPublicKey(pk *rsa.PublicKey) {
	this.PublKey = pk
}

// Receiving message from server
func (this *Client) ReadMessage() ([]interface{}, error) {
	reader := bufio.NewReader(this.connection)
	message := make([]byte, 0)

	var chunkSize = 2097152

	if this.PrivKey != nil {
		chunkSize *= rsacrypto.KeySize / 64
	}

	for {
		buf := make([]byte, chunkSize) // Chunk is equals 2 mib
		n, err := reader.Read(buf)
		if err != nil {
			return []interface{}{}, err
		}
		message = append(message, buf[:n]...)
		if n < chunkSize {
			break
		}
		if this.PrivKey != nil {
			_, err := rsacrypto.DecryptWithPrivateKey(message, this.PrivKey)
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

	if this.PrivKey != nil {
		msg, err := rsacrypto.DecryptWithPrivateKey(message, this.PrivKey)
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
func (this *Client) ListToMessage(list []interface{}) ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(list)
	if err != nil {
		return []byte{}, err
	}
	ret := buff.Bytes()
	if this.PublKey != nil {
		enc, err := rsacrypto.EncryptWithPublicKey(ret, this.PublKey)
		if err != nil {
			return []byte{}, err
		}
		return enc, nil
	}
	return ret, nil
}

//=== import cycle problem ===\\

// Print start menu
func StarterMenu() {
	ml := utils.Logger{Prefix: ""}
	errl := utils.Logger{Prefix: "Error"}
	ml.Println("What do you like do?")
	ml.Println("1. Create server")
	ml.Println("2. Create client")
	ml.Println("")
	for {
		sel := ml.Input("Enter number: ")
		numb, err := strconv.Atoi(sel)
		if err != nil {
			errl.PPrintln("Invalid number!")
			continue
		}
		if numb < 1 || numb > 2 {
			errl.PPrintln("Enter 1 or 2!")
			continue
		}
		starterHandler(numb)
		break
	}
}

// Handling user input server or client mode
func starterHandler(sel int) {
	if sel == 1 {
		serv := server.Create(server.CollectServerData())
		serv.Run()
	} else {
		cl := Create(CollectClientData())
		cl.Run()
	}
}
