WeChat: cstutorcs
QQ: 749389476
Email: tutorcs@163.com
package remote

import (
	"fmt"
	"s23lab1/src/remote"
	"sync"
	"time"
)

type Object struct {
	mu    sync.Mutex
	ma    sync.RWMutex
	state1 bool
	state2 bool
}

func (O *Object) Add(i int, j int) (int, remote.RemoteObjectError) {
	res := i + j
	return res, remote.RemoteObjectError{}
}

func (O *Object) Concat(i int, j int) (string, remote.RemoteObjectError) {
	res := fmt.Sprintf("%s%d%d", "test", i, j)
	return res, remote.RemoteObjectError{}
}

func (O *Object) SetState(i bool, j bool) remote.RemoteObjectError {
	O.mu.Lock()
	defer O.mu.Unlock()
	O.state1 = i
	O.state2 = j
	return remote.RemoteObjectError{}
}

func (O *Object) GetState() (bool, bool, remote.RemoteObjectError) {
	return O.state1, O.state2, remote.RemoteObjectError{}
}

type testInterface struct {
	Add      (func(int, int) (int, remote.RemoteObjectError))
	Concat   (func(int, int) (string, remote.RemoteObjectError))
	SetState (func(bool, bool) remote.RemoteObjectError)
	GetState (func() (bool, bool, remote.RemoteObjectError))
}

func main() {
	port := 15213

	serv, err := remote.NewService(&testInterface{}, &Object{}, port, true, true)
	if err != nil {
		fmt.Println("Error creating service:", err)
		return
	}

	if err := serv.Start(); err != nil {
		fmt.Println("Error starting service:", err)
	}
	// done <- true
	// // Wait for the service to finish
	// <-done

	println("sleeping")
	time.Sleep(1000000000000000000)

	// Stop the service after it's finished
	serv.Stop()
}
