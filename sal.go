// L'act

package main

import (
    "fmt"
    "os"
    "syscall"
    "os/signal"
    "runtime"
    //"net/http"
    //_ "net/http/pprof"

    "sal/network"
    "sal/proto"
)


func handleSig() {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)

    for sig := range c {
        fmt.Println("__handle__signal__", sig)
        return
    }
}


func main() {
    runtime.GOMAXPROCS(runtime.NumCPU())

    //go func() {
    //    log.Println(http.ListenAndServe("localhost:6061", nil))
    //}()

    sal_recv_chan := make(chan *proto.SalPack, 10240)
    sal_send_chan := make(chan *proto.SalPack, 10240)
    sal_bakend := network.NewSalBackend(sal_recv_chan, sal_send_chan)
    go sal_bakend.Start()
    sal_frontend := network.NewSalFrontend(sal_recv_chan, sal_send_chan)
    go sal_frontend.Start()

    handleSig()
}

