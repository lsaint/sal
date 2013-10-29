package network

/*
#cgo CFLAGS: -I /home/lsaint/go/src/sal/c
#cgo LDFLAGS: -L /home/lsaint/go/src/sal/ -lopenyy_sal
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "sal-c.h"
#include "sal-events-c.h"
*/
import "C"

import (
    "fmt"
    "log"
    "math"
    "unsafe"
    "strings"

    pb "code.google.com/p/goprotobuf/proto"
    
    "sal/conf"
    "sal/proto"
)


type SalBackend struct {
    sal             *C.openyy_SAL_t
    recvChan        chan *proto.SalPack
    sendChan        chan *proto.SalPack
}

func NewSalBackend(rc, sd chan *proto.SalPack) *SalBackend {
    key := C.CString(conf.CF.Key)
    defer C.free(unsafe.Pointer(key))
    sal := C.openyy_SAL_New(10088, key)
    if C.openyy_SAL_Init(sal) < 0 {
        panic("Init sal error")
    }
    return &SalBackend{sal: sal, recvChan: rc, sendChan: sd}
}

func (this *SalBackend) Start() {
    if C.openyy_SAL_Start(this.sal) < 0 {
        panic("Start sal error")
    }
    log.Println("SalBackend running")

    ev_chan := make(chan *C.openyy_SALEvent_t)
    done := make(chan bool)

    go this.waitEvent(ev_chan, done)
    this.handleEvent(ev_chan, done)
}

func (this *SalBackend) waitEvent(c chan *C.openyy_SALEvent_t, d chan bool) {
    for {
        ev := &C.openyy_SALEvent_t{}
        if C.openyy_SAL_WaitEvent(this.sal, &ev, -1) < 0 {
            fmt.Println("Wait sal event error")
            return
        }
        c <- ev
        <-d
        C.openyy_SALEvent_Free(ev)
    }
}

func (this *SalBackend) handleEvent(c chan *C.openyy_SALEvent_t, d chan bool) {
    for ev := range c {
        ev_type := C.openyy_SALEvent_GetType(ev) 
        switch ev_type {
            case C._SAL_LOG_EVENT_TP:
                this.handleLog(ev)

            case C._SAL_SUBSCRIBE_HASH_CHANNEL_RES_EVENT_TP:
                this.handleSubscribeRep(ev)

            case C._SAL_LOGIN_RES_EVENT_TP:
                this.handleLoginRep(ev)

            case C._SAL_USER_MSG_EVENT_TP:
                this.handleUserMsg(ev)

            case C._SAL_USER_OUT_CHANNEL_EVENT_TP:
                this.handleLogout(ev)

            default:
                fmt.Println("Sal event:", ev_type)
        }
        d <- true
    }
}

func (this *SalBackend) handleLogout(ev *C.openyy_SALEvent_t) {
    fmt.Println("handleLogout")
    var top_ch C.uint
    vv := C.openyy_SALUIntVectorVector_New()
    C.openyy_SALUserOutChannelEvent_Datas(ev, &top_ch, &vv)
    l := C.openyy_SALUIntVectorVector_Size(vv)
    for i:=C.uint(0); i<l; i++ {
        v := C.openyy_SALUIntVector_New() 
        C.openyy_SALUIntVectorVector_Get(vv, i, v)    
        if int(C.openyy_SALUIntVector_Size(v)) == 2 {
            var sub, uid C.uint
            C.openyy_SALUIntVector_Get(v, 0, &sub)
            C.openyy_SALUIntVector_Get(v, 1, &uid)
            fmt.Println("out channel", top_ch, sub, uid)
        }
        C.openyy_SALUIntVector_Destroy(v)
    }
    C.openyy_SALUIntVectorVector_Destroy(vv)
}

func (this *SalBackend) handleLog(ev *C.openyy_SALEvent_t) {
    var tm C.time_t
    var level_name, msg *C.char
    C.openyy_SALLogEvent_Datas(ev, &tm, &level_name, &msg)
    go_msg := C.GoString(msg)
    if !strings.Contains(go_msg, "sal message from client") {
        log.Println("[SAL]", C.GoString(level_name), go_msg)
    }
}

func (this *SalBackend) handleSubscribeRep(ev *C.openyy_SALEvent_t) {
    var sub_chns, unsub_chns *C.uint
    var sub_count, unsub_count, min, max C.uint
    C.openyy_SALSubscribeHashChannelResEvent_Datas(ev, &sub_chns, &sub_count, 
        &unsub_chns, &unsub_count, &min, &max)
    fmt.Println("subscribe count:", sub_chns, sub_count, min, max)
}

func (this *SalBackend) handleLoginRep(ev *C.openyy_SALEvent_t) {
    var info *C.char
    C.openyy_SALLoginResEvent_Datas(ev, &info)
    fmt.Println("[LOGIN_REP]", C.GoString(info))

    this.subscribe()

    go this.parseAndSendMsgToSal()
}

func (this *SalBackend) subscribe() {
    C.openyy_SAL_SubscribeHashChRange(this.sal, 0, math.MaxUint32, C._SAL_HASH_CH_RANGE_NONE)
}

func (this *SalBackend) parseAndSendMsgToSal() {
    for pack := range this.sendChan {
        sid := C.uint(pack.GetSid())
        sub := C.uint(pack.GetSub())
        uids := pack.GetUids()
        msg := pack.GetBin()
        msg_len := C.uint(len(msg))

        if sub != 0 {
            C.openyy_SAL_BcMsgToSubCh(this.sal, sid, sub, 0,
                   (*C.char)(unsafe.Pointer(&msg[0])), msg_len )
            continue
        }

        if len(uids) != 0 {
            C.openyy_SAL_MulticastMsgToUsers(this.sal, sid, 0,
                    (*C.uint)(unsafe.Pointer(&uids[0])), C.uint(len(uids)),
                    (*C.char)(unsafe.Pointer(&msg[0])), msg_len)
            continue
        }

        C.openyy_SAL_BcMsgToTopCh(this.sal, sid, 0,
                    (*C.char)(unsafe.Pointer(&msg[0])), msg_len)
    }
}

func (this *SalBackend) handleUserMsg(ev *C.openyy_SALEvent_t) {
    var top_ch, uid, msg_size C.uint
    var msg *C.char
    C.openyy_SALUserMsgEvent_Datas(ev, nil, &top_ch, &uid, &msg, &msg_size);
    //if msg_size < 8 {return}

    b := C.GoBytes(unsafe.Pointer(msg), C.int(msg_size))
    //fmt.Println("[USR_MSG]", top_ch, uid, msg_size)
    uids := make([]uint32, 1, 1)
    uids[0] = uint32(uid)
    pack := &proto.SalPack{Uids: uids, Sid: pb.Uint32(uint32(top_ch)), Bin: b}
    select {
        case this.recvChan <- pack:

        default:
            fmt.Println("user recive buff overflow")
    }
}

