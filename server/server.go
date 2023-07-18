package server

import (
	"GijzaFiler/utils"
	"bufio"
	"bytes"
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

const DEFAULTPORT int = 5416

func CollectServerData() (int, string, []string) {
	ml := utils.Logger{Prefix: ""}
	errl := utils.Logger{Prefix: "Error"}
	var Port int
	var Dirname string
	var Passwords []string
	for {
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
	}
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
	for {
		password := ml.Input("Enter password #" + fmt.Sprint(len(Passwords)+1) + ": ")
		if password == "" {
			break
		} else {
			Passwords = append(Passwords, password)
		}
	}
	return Port, Dirname, Passwords
}

type Server struct {
	Port             int
	Directory        string
	Passwords        []string
	BytesLimit       int
	ConnectionsLimit int
	ConnectionCount  int
	listener         net.Listener
}

func Create(port int, directory string, passwords []string) Server {
	return Server{Port: port, Directory: directory, Passwords: passwords, BytesLimit: 2048, ConnectionsLimit: 20, ConnectionCount: 0}
}

func (this *Server) Run() {
	inf := utils.Logger{Prefix: "server"}
	errl := utils.Logger{Prefix: "error"}
	inf.PPrintln("Server starting on port " + fmt.Sprint(this.Port))
	listen, err := net.Listen("tcp", ":"+fmt.Sprint(this.Port))
	if err != nil {
		errl.PPrintln("An error occurred while creating the server: " + err.Error())
		port, directory, passwords := CollectServerData()
		this.Port = port
		this.Directory = directory
		this.Passwords = passwords
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
		if this.ConnectionCount+1 > this.ConnectionsLimit && this.ConnectionsLimit != -1 {
			con.Close()
		} else {
			this.ConnectionCount++
			go this.ClientHandler(con)
		}
	}
}

func (this Server) MinusConnection() {
	this.ConnectionCount--
}

func (this Server) ClientHandler(con net.Conn) {
	defer this.MinusConnection()
	inf := utils.Logger{Prefix: "server"}
	errl := utils.Logger{Prefix: "error"}
	inf.PPrintln(con.RemoteAddr().String() + " connected!")
	defer con.Close()
	defer inf.PPrintln(con.RemoteAddr().String() + " disconnected!")
	con.SetDeadline(time.Now().Add(2 * time.Second))
	var authed bool = false
	for {
		req, err := this.ReadMessage(con)
		if err != nil {
			errl.PPrintln("Receiving message error: " + err.Error())
			return
		}
		if !authed {
			if req[0] == "connect" {
				con.SetDeadline(time.Time{})
				if len(this.Passwords) == 0 {
					res, _ := this.ListToMessage([]interface{}{"success"})
					_, err = con.Write(res)
					if err != nil {
						errl.PPrintln("Sending error: " + err.Error())
						return
					}
					authed = true
				} else {
					res, _ := this.ListToMessage([]interface{}{"enter_password", len(this.Passwords)})
					_, err = con.Write(res)
					if err != nil {
						errl.PPrintln("Sending error: " + err.Error())
						return
					}
				}
			} else if req[0] == "password" && len(this.Passwords) != 0 {
				if len(req)-1 != len(this.Passwords) {
					res, _ := this.ListToMessage([]interface{}{"fail"})
					_, err = con.Write(res)
					if err != nil {
						errl.PPrintln("Sending error: " + err.Error())
						return
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
						res, _ := this.ListToMessage([]interface{}{"success"})
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						inf.PPrintln(con.RemoteAddr().String() + " signed in!")
						authed = true
					} else {
						res, _ := this.ListToMessage([]interface{}{"fail"})
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
					}
				}
			} else {
				errl.PPrintln("Client sent unknown command")
				return
			}
		} else {
			if req[0] == "get_folders" && len(req) == 2 {
				if foldname, ok := req[1].(string); ok {
					var splitted []string = strings.Split(strings.ReplaceAll(foldname, "\\", "/"), "/")
					var success bool = true
					if foldname != "" {
						for i, a := range splitted {
							stat, err := os.ReadDir(path.Join(this.Directory, strings.Join(splitted[:i], "/")))
							if err != nil {
								res, _ := this.ListToMessage([]interface{}{"fail", err.Error()})
								_, err = con.Write(res)
								if err != nil {
									errl.PPrintln("Sending error: " + err.Error())
									return
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
						re, _ := this.ListToMessage(res)
						_, err = con.Write(re)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
					}
					res := []interface{}{"success"}
					stat, err := os.ReadDir(path.Join(this.Directory, foldname))
					if err != nil {
						res, _ := this.ListToMessage([]interface{}{"fail", err.Error()})
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
					}
					for _, nm := range stat {
						if nm.IsDir() {
							res = append(res, nm.Name())
						}
					}
					re, _ := this.ListToMessage(res)
					_, err = con.Write(re)
					if err != nil {
						errl.PPrintln("Sending error: " + err.Error())
						return
					}
				} else {
					errl.PPrintln("Client sent unknown command")
					return
				}
			} else if req[0] == "get_files" && len(req) == 2 {
				if foldname, ok := req[1].(string); ok {
					var splitted []string = strings.Split(strings.ReplaceAll(foldname, "\\", "/"), "/")
					var success bool = true
					if foldname != "" {
						for i, a := range splitted {
							stat, err := os.ReadDir(path.Join(this.Directory, strings.Join(splitted[:i], "/")))
							if err != nil {
								res, _ := this.ListToMessage([]interface{}{"fail", err.Error()})
								_, err = con.Write(res)
								if err != nil {
									errl.PPrintln("Sending error: " + err.Error())
									return
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
						re, _ := this.ListToMessage(res)
						_, err = con.Write(re)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
					}
					res := []interface{}{"success"}
					stat, err := os.ReadDir(path.Join(this.Directory, foldname))
					if err != nil {
						res, _ := this.ListToMessage([]interface{}{"fail", err.Error()})
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
					}
					for _, nm := range stat {
						if !nm.IsDir() {
							res = append(res, nm.Name())
						}
					}
					re, _ := this.ListToMessage(res)
					_, err = con.Write(re)
					if err != nil {
						errl.PPrintln("Sending error: " + err.Error())
						return
					}
				} else {
					errl.PPrintln("Client sent unknown command")
					return
				}
			} else if req[0] == "download" && len(req) == 2 {
				if foldname, ok := req[1].(string); ok {
					if foldname == "." {
						res := []interface{}{"success"}
						dirls := []string{}
						fils := []string{}
						iter_folder(this.Directory, "", &dirls, &fils)
						res = append(res, dirls)
						res = append(res, fils)
						re, _ := this.ListToMessage(res)
						fmt.Println(res)
						_, err = con.Write(re)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
					}
					foldname = strings.ReplaceAll(foldname, "\\", "/")
					var splitted []string = strings.Split(strings.ReplaceAll(foldname, "\\", "/"), "/")
					var success bool = true
					if foldname != "" {
						for i, a := range splitted[:len(splitted)-1] {
							stat, err := os.ReadDir(path.Join(this.Directory, strings.Join(splitted[:i], "/")))
							if err != nil {
								res, _ := this.ListToMessage([]interface{}{"fail", err.Error()})
								_, err = con.Write(res)
								if err != nil {
									errl.PPrintln("Sending error: " + err.Error())
									return
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
						re, _ := this.ListToMessage(res)
						_, err = con.Write(re)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
					}
					res := []interface{}{"success"}
					stat, err := os.ReadDir(path.Dir(path.Join(this.Directory, foldname)))
					if err != nil {
						res, _ := this.ListToMessage([]interface{}{"fail", err.Error()})
						_, err = con.Write(res)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
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
								iter_folder(path.Join(this.Directory, foldname), splitted[len(splitted)-1], &dirls, &fils)
								res = append(res, dirls)
								res = append(res, fils)
							} else {
								res = append(res, "file")
								cont, err := os.ReadFile(path.Join(this.Directory, foldname))
								if err != nil {
									res := []interface{}{"fail", "the file cannot be read"}
									re, _ := this.ListToMessage(res)
									_, err = con.Write(re)
									if err != nil {
										errl.PPrintln("Sending error: " + err.Error())
										return
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
						continue
					}
					if finded {
						re, _ := this.ListToMessage(res)
						_, err = con.Write(re)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
					} else {
						res := []interface{}{"fail", "folder/file not found!"}
						re, _ := this.ListToMessage(res)
						_, err = con.Write(re)
						if err != nil {
							errl.PPrintln("Sending error: " + err.Error())
							return
						}
						continue
					}
				} else {
					errl.PPrintln("Client sent unknown command")
					return
				}
			} else {
				errl.PPrintln("Client sent unknown command")
				return
			}
		}
	}
}

func iter_folder(path string, write_as string, dirls *[]string, fils *[]string) {
	p, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, u := range p {
		if u.IsDir() {
			*dirls = append(*dirls, filepath.Join(write_as, u.Name()))
			iter_folder(filepath.Join(path, u.Name()), filepath.Join(write_as, u.Name()), dirls, fils)
		} else {
			*fils = append(*fils, filepath.Join(write_as, u.Name()))
		}
	}
}

func (this Server) ReadMessage(client net.Conn) ([]interface{}, error) {
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

func (this Server) ListToMessage(list []interface{}) ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(list)
	if err != nil {
		return []byte{}, err
	}
	return buff.Bytes(), nil
}
