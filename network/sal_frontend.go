package network

import (
    "fmt"
    "net"
    "encoding/binary"

    pb "code.google.com/p/goprotobuf/proto"

    "sal/proto"
)

const (
    URI_REGISTER        = 1
    URI_TRANSPORT       = 2
    URI_UNREGISTER      = 3
    LEN_URI             = 4
    FRONT_PORT          = ":3929"
)


type SalFrontend struct {
    buffChan        chan *ConnBuff
    agentConns      []*ClientConnection         
    FromSalChan     chan *proto.SalPack
    ToSalChan       chan *proto.SalPack
}

func NewSalFrontend(entry, exit chan *proto.SalPack) *SalFrontend {
    gs := &SalFrontend{buffChan: make(chan *ConnBuff),
                    agentConns:  make([]*ClientConnection, 0, 1),
                    FromSalChan: entry,
                    ToSalChan:  exit}
    go gs.parse()
    return gs
}


func (this *SalFrontend) Start() {
    ln, err := net.Listen("tcp", FRONT_PORT)                                                                            
    if err != nil {
        fmt.Println("Listen err", err)
        return
    }
    fmt.Println("SalFrontend running", FRONT_PORT)
    for {
        conn, err := ln.Accept()
        if err != nil {
            fmt.Println("Accept error", err)
            continue
        }
        fmt.Println("sal agent connected")
        go this.acceptConn(conn)
    }
}

func (this *SalFrontend) acceptConn(conn net.Conn) {
    cliConn := NewClientConnection(conn)
    for {
        if buff_body, ok := cliConn.duplexReadBody(); ok {
            this.buffChan <- &ConnBuff{cliConn, buff_body}
            continue
        }
        this.buffChan <- &ConnBuff{cliConn, nil}
        break
    }
    fmt.Println("sal agent disconnect")
}

func (this *SalFrontend) parse() {
    for {
        select {
            case conn_buff := <-this.buffChan :
                msg := conn_buff.buff
                conn := conn_buff.conn

                if msg == nil {
                    this.unregister(conn)
                    continue
                }

                uri := binary.LittleEndian.Uint32(msg[:LEN_URI])
                switch uri {
                case URI_REGISTER:
                    this.register(msg[LEN_URI:], conn)
                case URI_TRANSPORT:
                    this.comein(msg[LEN_URI:])
                default:
                    this.unregister(conn)
                }

            case pack := <-this.FromSalChan :
                fmt.Println("FromSalChan", pack)
                this.comeout(pack)
        }
    }
}

func (this *SalFrontend) unpack(b []byte) (msg *proto.SalPack, err error) {
    msg = &proto.SalPack{}
    if err = pb.Unmarshal(b, msg); err != nil {
        fmt.Println("pb Unmarshal", err)
    }
    return
}

func (this *SalFrontend) comein(b []byte) {
    if msg, err := this.unpack(b); err == nil {
        this.ToSalChan <- msg
    }
}

func (this *SalFrontend) comeout(pack *proto.SalPack) {
    for _, conn := range this.agentConns {
        if data, err := pb.Marshal(pack); err == nil {
            uri_field := make([]byte, LEN_URI)
            binary.LittleEndian.PutUint32(uri_field, uint32(URI_TRANSPORT))
            data = append(uri_field, data...)
            fmt.Println("send to agent", len(data))
            conn.Send(data)
        }
    }
}

func (this *SalFrontend) register(b []byte, cc *ClientConnection) {
    fmt.Println("register")
    this.agentConns = append(this.agentConns, cc)
}

func (this *SalFrontend) unregister(cc *ClientConnection) {
    fmt.Println("unregister")
    for i, c := range this.agentConns {
        if cc == c {
            last := len(this.agentConns) - 1
            this.agentConns[i] = this.agentConns[last]
            this.agentConns = this.agentConns[:last]
        }
    }
}



