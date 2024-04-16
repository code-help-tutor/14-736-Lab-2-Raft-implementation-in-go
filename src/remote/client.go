WeChat: cstutorcs
QQ: 749389476
Email: tutorcs@163.com
package remote

import (
	"fmt"
	"s23lab1/src/remote"
	"strconv"
)

type testInterface struct {
	Add      (func(int, int) (int, remote.RemoteObjectError))
	Concat   (func(int, int) (string, remote.RemoteObjectError))
	SetState (func(bool, bool) remote.RemoteObjectError)
	GetState (func() (bool, bool, remote.RemoteObjectError))
}

func main() {
	port := 15213
	addr := "127.0.0.1:" + strconv.Itoa(port)

	serv := &testInterface{}
	err := remote.StubFactory(serv, addr, true, true)
	if err != nil {
		fmt.Println("Error creating stub:", err)
		return
	}

	if sum, err := serv.Add(3, 5); err.Err != "" {
		fmt.Println(err.Err)
	} else {
		fmt.Println(sum)
	}

	if str, err := serv.Concat(5, 6); err.Err != "" {
		fmt.Println(err.Err)
	} else {
		fmt.Println(str)
	}

	if err := serv.SetState(true, false); err.Err != "" {
		fmt.Println(err.Err)
	} 
	if state, state2, err := serv.GetState(); err.Err != "" {
		fmt.Println(err.Err)
	} else {
		fmt.Println(state, state2)
	}

}
