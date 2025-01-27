package persistence

import (
	"net/rpc"
	"os"
	"sync"

	"github.com/spacemeshos/post/remote_config"
)

func NewRemoteLabelsWriter(datadir string, index int, bitsPerLabel uint) (*RemoteWriter, error) {
	address, ok := os.LookupEnv("REMOTE_ADDR")
	if !ok {
		panic("no REMOTE_ADDR provided")
	}
	conn, err := rpc.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	err = conn.Call("RpcFileWriter.Open", remote_config.RemoteWriterOpenData{
		DataDir:      datadir,
		Index:        index,
		BitsPerLabel: bitsPerLabel,
	}, nil)
	if err != nil {
		return nil, err
	}
	wr := &RemoteWriter{writeBuf: make(chan []byte, 16), client: conn}
	go func() {
		for buf := range wr.writeBuf {
			wr.error = conn.Call("RpcFileWriter.Write", buf, nil)
			wr.remain.Done()
			if wr.error != nil {
				return
			}
		}
	}()
	return wr, err

}

type RemoteWriter struct {
	writeBuf chan []byte
	client   *rpc.Client
	remain   sync.WaitGroup
	error    error
}

func (wr *RemoteWriter) Write(b []byte) error {
	wr.writeBuf <- b
	wr.remain.Add(1)
	return wr.error
}

func (wr *RemoteWriter) Flush() error {
	wr.remain.Wait()
	return wr.error
}

func (wr *RemoteWriter) NumLabelsWritten() (uint64, error) {
	var reply uint64
	err := wr.client.Call("RpcFileWriter.NumLabelsWritten", struct{}{}, &reply)
	return reply, err
}

func (wr *RemoteWriter) Truncate(numLabels uint64) error {
	return wr.client.Call("RpcFileWriter.Truncate", numLabels, nil)
}

func (wr *RemoteWriter) Close() error {
	wr.remain.Wait()
	if wr.error != nil {
		return wr.error
	}
	err := wr.client.Call("RpcFileWriter.Close", struct{}{}, nil)
	if err != nil {
		return err
	}
	return wr.client.Close()
}
