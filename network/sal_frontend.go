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
    sid2conn        map[uint32]*ClientConnection
    conn2sid        map[*ClientConnection]uint32
    SalEntry        chan *proto.SalPack
    SalExit         chan *proto.SalPack
}

func NewSalFrontend(entry, exit chan *proto.SalPack) *SalFrontend {
    gs := &SalFrontend{buffChan: make(chan *ConnBuff),
                    sid2conn: make(map[uint32]*ClientConnection),
                    conn2sid: make(map[*ClientConnection]uint32),
                    SalEntry: entry,
                    SalExit:  exit}
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

            case pack := <-this.SalExit :
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
        this.SalEntry <- msg
    }
}

func (this *SalFrontend) comeout(pack *proto.SalPack) {
    conn, exist := this.sid2conn[pack.GetSid()]
    if !exist {
        return
    }
    if data, err := pb.Marshal(pack); err == nil {
        uri_field := make([]byte, LEN_URI)
        binary.LittleEndian.PutUint32(uri_field, uint32(URI_TRANSPORT))
        data = append(data, uri_field...)
        conn.Send(data)
    }
}

func (this *SalFrontend) register(b []byte, cc *ClientConnection) {
    if msg, err := this.unpack(b); err == nil {
        sid := msg.GetSid()    
        this.sid2conn[sid] = cc
        this.conn2sid[cc] = sid
    }
}

func (this *SalFrontend) unregister(cc *ClientConnection) {
    sid := this.conn2sid[cc]
    delete(this.sid2conn, sid)
    delete(this.conn2sid, cc)
}



