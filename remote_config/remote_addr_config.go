package remote_config

import "net"

var TargetConnectAddr = &net.TCPAddr{
	IP: net.IP{100, 107, 163, 58},
	//IP:   net.IP{100, 94, 63, 89},
	Port: 25446,
}

type RemoteCMD uint8

type RemoteWriterOpenData struct {
	DataDir      string
	Index        int
	BitsPerLabel uint
}
