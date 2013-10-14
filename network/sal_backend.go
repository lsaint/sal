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
    "unsafe"
    //"reflect"
    "math"

    pb "code.google.com/p/goprotobuf/proto"
    
    "sal/proto"
)


type SalBackend struct {
    sal             *C.openyy_SAL_t
    recvChan        chan *proto.SalPack
    sendChan        chan *proto.SalPack
}

func NewSalBackend(rc, sd chan *proto.SalPack) *SalBackend {
    key := C.CString("b218dd6a62724b799cc0c39625b2f386a5506f82")
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
    fmt.Println("SalBackend running")

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
                //this.handleLog(ev)

            case C._SAL_SUBSCRIBE_HASH_CHANNEL_RES_EVENT_TP:
                this.handleSubscribeRep(ev)

            case C._SAL_LOGIN_RES_EVENT_TP:
                this.handleLoginRep(ev)

            case C._SAL_USER_MSG_EVENT_TP:
                this.handleUserMsg(ev)

            default:
                fmt.Println("Sal event:", ev_type)
        }
        d <- true
    }
}

func (this *SalBackend) handleLog(ev *C.openyy_SALEvent_t) {
    var tm C.time_t
    var level_name, msg *C.char
    C.openyy_SALLogEvent_Datas(ev, &tm, &level_name, &msg)
    fmt.Println("[LOG]", tm, C.GoString(level_name), C.GoString(msg))
}

func (this *SalBackend) handleSubscribeRep(ev *C.openyy_SALEvent_t) {
    var sub_chns, unsub_chns *C.uint
    var sub_count, unsub_count, min, max C.uint
    C.openyy_SALSubscribeHashChannelResEvent_Datas(ev, &sub_chns, &sub_count, 
        &unsub_chns, &unsub_count, &min, &max)
    fmt.Println("subscribe count:", sub_chns, sub_count, min, max)
    
    //var theGoSlice []*C.uint
    //sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&theGoSlice)))
    //sliceHeader.Cap = int(sub_count)
    //sliceHeader.Len = int(sub_count)
    //sliceHeader.Data = uintptr(unsafe.Pointer(&sub_chns))
    //fmt.Println(theGoSlice, *theGoSlice[0])
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
            return
        }

        if len(uids) != 0 {
            C.openyy_SAL_MulticastMsgToUsers(this.sal, sid, 0,
                    (*C.uint)(unsafe.Pointer(&uids[0])), C.uint(len(uids)),
                    (*C.char)(unsafe.Pointer(&msg[0])), msg_len)
            return
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
    fmt.Println("[USR_MSG]", top_ch, uid, msg_size, string(b))
    uids := make([]uint32, 1, 1)
    uids[0] = uint32(uid)
    pack := &proto.SalPack{Uids: uids, Sid: pb.Uint32(uint32(top_ch)), Bin: b}
    select {
        case this.recvChan <- pack:

        default:
            fmt.Println("user recive buff overflow")
    }
}

