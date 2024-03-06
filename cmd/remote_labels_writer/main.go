package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"

	"github.com/spacemeshos/post/remote_config"
	"github.com/spacemeshos/post/shared"
)

var fileWriter *FileWriter

func main() {
	ln, err := net.ListenTCP("tcp", remote_config.TargetConnectAddr)
	if err != nil {
		panic(err)
	}
	rpc.Register(new(RpcFileWriter))
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			panic(err)
		}
		log.Println("Accepted connection", conn.RemoteAddr().String())
		rpc.ServeConn(conn)
	}
}

type RpcFileWriter struct{}

func (w *RpcFileWriter) Open(data remote_config.RemoteWriterOpenData, _ *struct{}) error {
	var err error
	log.Println("Opening file writer", data.DataDir, data.Index, data.BitsPerLabel)
	fileWriter, err = NewLabelsWriter(data.DataDir, data.Index, data.BitsPerLabel)
	return err
}

func (w *RpcFileWriter) Write(b []byte, reply *bool) error {
	return fileWriter.Write(b)
}

func (w *RpcFileWriter) Flush(_ struct{}, _ *struct{}) error {
	return fileWriter.Flush()
}

func (w *RpcFileWriter) NumLabelsWritten(_ struct{}, reply *uint64) error {
	numLabels, err := fileWriter.NumLabelsWritten()
	*reply = numLabels
	return err
}

func (w *RpcFileWriter) Truncate(numLabels uint64, _ *struct{}) error {
	return fileWriter.Truncate(numLabels)
}

func (w *RpcFileWriter) Close(_ struct{}, _ *struct{}) error {
	return fileWriter.Close()
}

type FileWriter struct {
	file *os.File
	buf  *bufio.Writer

	bitsPerLabel uint
}

func NewLabelsWriter(datadir string, index int, bitsPerLabel uint) (*FileWriter, error) {
	if err := os.MkdirAll(datadir, shared.OwnerReadWriteExec); err != nil {
		return nil, err
	}

	filename := filepath.Join(datadir, shared.InitFileName(index))
	return NewFileWriter(filename, bitsPerLabel)
}

func NewFileWriter(filename string, bitsPerLabel uint) (*FileWriter, error) {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, shared.OwnerReadWrite)
	if err != nil {
		return nil, err
	}
	f.Seek(0, io.SeekEnd)
	return &FileWriter{
		file:         f,
		buf:          bufio.NewWriter(f),
		bitsPerLabel: bitsPerLabel,
	}, nil
}

func (w *FileWriter) Write(b []byte) error {
	_, err := w.buf.Write(b)
	return err
}

func (w *FileWriter) Flush() error {
	if err := w.buf.Flush(); err != nil {
		return fmt.Errorf("failed to flush disk writer: %w", err)
	}

	return nil
}

func (w *FileWriter) NumLabelsWritten() (uint64, error) {
	info, err := w.file.Stat()
	if err != nil {
		return 0, err
	}

	return uint64(info.Size()) * 8 / uint64(w.bitsPerLabel), nil
}

func (w *FileWriter) Truncate(numLabels uint64) error {
	bitSize := numLabels * uint64(w.bitsPerLabel)
	if bitSize%8 != 0 {
		return fmt.Errorf("invalid `numLabels`; expected: evenly divisible by 8 (alone, or when multiplied by `labelSize`), given: %d", numLabels)
	}

	size := int64(bitSize / 8)
	if err := w.file.Truncate(size); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}
	w.file.Sync()
	return nil
}

func (w *FileWriter) Close() error {
	if err := w.buf.Flush(); err != nil {
		return err
	}

	return w.file.Close()
}
