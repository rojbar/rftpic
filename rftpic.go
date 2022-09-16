package rftpic

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	utils "github.com/rojbar/rftpiu"
)

const BUFFERSIZE = 4096

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
	conn, res, errS := obtainConnection("RFTP > 1.0 ACTION: SEND SIZE: "+size+" "+ext+" CHANNEL: "+channel+";", port, domain)
	if errS != nil {
		return errS
	}
	if res != "OK" {
		return errors.New(res)
	}

	defer conn.Close()

	writer := bufio.NewWriter(conn)

	buffer := make([]byte, BUFFERSIZE)
	chunks, sizeLastChunk := utils.CalculateChunksToSendExactly(int(sizeInt), BUFFERSIZE)
	remainderBuffer := make([]byte, sizeLastChunk)

	loops := chunks
	if sizeLastChunk != 0 {
		loops += 1
	}

	for i := 0; i < int(loops); i++ {
		auxBuffer := buffer
		if i == chunks {
			auxBuffer = remainderBuffer
		}

		errRnWf := utils.ReadThenWrite(file, *writer, auxBuffer)
		if errRnWf != nil {
			return errRnWf
		}
	}

	//here we check that server recieved the file correctly
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

func Subscribe(port string, domain string, channel string) error {

	for i := 0; i < 50; i++ {
		err := os.MkdirAll("recieve/channels/"+strconv.Itoa(i), 0750)
		if err != nil && !os.IsExist(err) {
			return err
		}
	}

	// inform the server we want to subscribe to certain channel
	conn, res, errS := obtainConnection("RFTP > 1.0 ACTION: SUBSCRIBE CHANNEL: "+channel+";", port, domain)
	if errS != nil {
		return errS
	}
	if res != "OK" {
		return errors.New(res)
	}
	handleSubscription(conn)
	return nil
}

func handleSubscription(conn net.Conn) error {
	defer conn.Close()
	for {
		message, errNf := utils.ReadMessage(conn)
		if errNf != nil {
			continue
		}
		// we check the message is for recieving a file
		// if it is we start recieving the file
		value, errAct := utils.GetKey(message, "ACTION")
		if errAct != nil || value != "SEND" {
			fmt.Println("trying to find ACTION SEND,", errAct)
			continue
		}

		channelName, errCN := utils.GetKey(message, "CHANNEL")
		value, errSz := utils.GetKey(message, "SIZE")
		fileSize, errAtoi := strconv.Atoi(value)
		if errCN != nil || errSz != nil || errAtoi != nil || fileSize <= 0 {
			fmt.Println(errCN, errSz, errAtoi)
			continue
		}

		extension, errExt := utils.GetKey(message, "EXTENSION")
		if errExt != nil {
			extension = " "
			fmt.Println("couldn't find extension", errExt)
		}

		buffer := make([]byte, BUFFERSIZE)
		chunks, sizeLastChunk := utils.CalculateChunksToSendExactly(fileSize, BUFFERSIZE)
		remainderBuffer := make([]byte, sizeLastChunk)

		loops := chunks
		if sizeLastChunk != 0 {
			loops += 1
		}

		file, errC := os.Create("recieve/channels/" + channelName + "/" + uuid.NewString() + "." + extension)
		if errC != nil {
			fmt.Println("couldn't create file for recieving", errC)
			continue
		}

		errI := utils.SendMessage(conn, "RFTP > 1.0 STATUS: OK;")
		if errI != nil {
			fmt.Println("coudln't inform server we ok for transfering file", errI)
		}

		// we recieve the file
		writer := bufio.NewWriter(file)
		for i := 0; i < loops; i++ {
			auxbuffer := buffer
			if i == chunks {
				auxbuffer = remainderBuffer
			}
			errRnWf := utils.ReadThenWrite(conn, *writer, auxbuffer)
			if errRnWf != nil {
				fmt.Println("error when reading from net and writing to file", errRnWf)
				utils.SendMessage(conn, "RFTP > 1.0 STATUS: NOT OK;")
				break
			}
		}
		utils.SendMessage(conn, "RFTP > 1.0 STATUS: OK;")
		file.Close()
	}
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
