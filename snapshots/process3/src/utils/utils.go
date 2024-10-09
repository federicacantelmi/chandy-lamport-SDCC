package utils

import "net"

var ProcessList map[int]*net.TCPAddr // processi attivi nel sistema
