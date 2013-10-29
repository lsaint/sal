package conf

import (
    "fmt"
    "strings"
    "io/ioutil"
)


var CF *Config
func init() {
    CF = NewConfig()
}

type Config struct {
    Key     string
}

func NewConfig() *Config {
    f, err := ioutil.ReadFile("./conf/config.txt")
    if err != nil {
        fmt.Println(err)
        panic("read config err")
    }
    lt := strings.Split(string(f), "\n")    
    fmt.Println("[CF]", lt)
    cf := &Config{}
    cf.Key = lt[0]
    return cf
}
