package rftpic

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	utils "github.com/rojbar/rftpiu"
)

/**
	1. send a message informing that a file is going to be send
	2. recieve confirmation and connection
	3. initiate transfer
	4. recieve confirmation all ok from server
	5. end transmission either client or server
	6. inform end user all ok
**/
func SendFile(port string, domain string, channel string, filePath string) error {
	file, errO := os.Open(filePath)
	if errO != nil {
		return errO
	}
	defer file.Close()

	fileInfo, errFS := file.Stat()
	if errFS != nil {
		return errFS
	}
	sizeInt := fileInfo.Size()
	size := strconv.Itoa(int(sizeInt))
	extension := fileInfo.Name()
	ext := "EXTENSION: "
	_, after, found := strings.Cut(extension, ".")
	if found {
		ext = "EXTENSION: " + after
	}

	// inform the server we are gonna send a file and recieve a net.Conn to handle that
	conn, res, errS := obtainConnection("SFTP > 1.0 ACTION: SEND SIZE: "+size+" "+ext+";", port, domain)
	if errS != nil {
		return errS
	}
	if res != "OK" {
		return errors.New(res)
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(file)

	buffer := make([]byte, 4096)
	for {
		_, errP := reader.Read(buffer)
		if errP != nil {
			if errP == io.EOF {
				break
			}
			if errP != nil {
				return errP
			}
		}

		_, errW := writer.Write(buffer)
		if errW != nil {
			return errW
		}
		errF := writer.Flush()
		if errF != nil {
			return errF
		}
	}
	//here we check that server recieved file correctly
	message, errMes := utils.ReadMessage(conn)
	if errMes != nil {
		print(errMes)
		return errMes
	}
	//here we parse
	status, errSt := utils.GetKey(message, "STATUS")
	if errSt != nil {
		return errSt
	}

	if status != "OK" {
		return errors.New(status)
	}

	return nil
}

func Subscribe(message string, port string, domain string, channel string) error {
	// conn, errS := send(message, port, domain) // here we tell the server that we want to create a subscription, the channel is in the payload, we recieve a net.Conn to handle the subscription
	// // check(errS)
	// go handleChannelSubscription(conn, channel)
	return nil
}

// OK
func obtainConnection(message string, port string, domain string) (net.Conn, string, error) {
	conn, err := net.Dial("tcp", domain+":"+port)
	if err != nil {
		return nil, "", err
	}

	errW := utils.SendMessage(conn, message)
	if errW != nil {
		return nil, "", errW
	}

	response, errR := utils.ReadMessage(conn)
	if errR != nil {
		return nil, "", errR
	}

	//here we parse
	status, errS := utils.GetKey(response, "STATUS")
	if errS != nil {
		return nil, "", errS
	}
	return conn, status, nil
}
