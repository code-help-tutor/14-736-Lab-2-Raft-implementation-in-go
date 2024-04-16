WeChat: cstutorcs
QQ: 749389476
Email: tutorcs@163.com
// support for generic Remote Object services over sockets
// including a socket wrapper that can drop and/or delay messages arbitrarily
// works with any* objects that can be gob-encoded for serialization
//
// the LeakySocket wrapper for net.Conn is provided in its entirety, and should
// not be changed, though you may extend it with additional helper functions as
// desired.  it is used directly by the test code.
//
// the RemoteObjectError type is also provided in its entirety, and should not
// be changed.
//
// suggested RequestMsg and ReplyMsg types are included to get you started,
// but they are only used internally to the remote library, so you can use
// something else if you prefer
//
// the Service type represents the callee that manages remote objects, invokes
// calls from callers, and returns suitable results and/or remote errors
//
// the StubFactory converts a struct of function declarations into a functional
// caller stub by automatically populating the function definitions.
//
// USAGE:
// the desired usage of this library is as follows (not showing all error-checking
// for clarity and brevity):
//
//	example ServiceInterface known to both client and server, defined as
//	type ServiceInterface struct {
//	    ExampleMethod func(int, int) (int, remote.RemoteObjectError)
//	}
//
//	1. server-side program calls NewService with interface and connection details, e.g.,
//	   obj := &ServiceObject{}
//	   srvc, err := remote.NewService(&ServiceInterface{}, obj, 9999, true, true)
//
//	2. client-side program calls StubFactory, e.g.,
//	   stub := &ServiceInterface{}
//	   err := StubFactory(stub, 9999, true, true)
//
//	3. client makes calls, e.g.,
//	   n, roe := stub.ExampleMethod(7, 14736)
package remote

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"time"
)

// LeakySocket
//
// LeakySocket is a wrapper for a net.Conn connection that emulates
// transmission delays and random packet loss. it has its own send
// and receive functions that together mimic an unreliable connection
// that can be customized to stress-test remote service interactions.

type LeakySocket struct {
	s         net.Conn
	isLossy   bool
	lossRate  float32
	msTimeout int
	usTimeout int
	isDelayed bool
	msDelay   int
	usDelay   int
}

// builder for a LeakySocket given a normal socket and indicators
// of whether the connection should experience loss and delay.
// uses default loss and delay values that can be changed using setters.
func NewLeakySocket(conn net.Conn, lossy bool, delayed bool) *LeakySocket {
	ls := &LeakySocket{}
	ls.s = conn
	ls.isLossy = lossy
	ls.isDelayed = delayed
	ls.msDelay = 2
	ls.usDelay = 0
	ls.msTimeout = 500
	ls.usTimeout = 0
	
	ls.lossRate = 0.05

	return ls
}

func EncodeRequest(req *RequestMsg) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(req.Method)
	enc.Encode(len(req.Args))

	for _, item := range req.Args {
		enc.EncodeValue(item)
	}
	// enc.Encode(req.Method)
	// enc.Encode(req.Args)
	// enc.Encode(req)
	new_buf, _ := ioutil.ReadAll(&buf)
	return new_buf
}

func DecodeRequest(method_name string, method_typ reflect.Type, dec gob.Decoder) (*RequestMsg, error) {
	req_msg := RequestMsg{}
	var num_args int
	req_msg.Method = method_name

	if err := dec.Decode(&num_args); err != nil {
		log.Println(err)
	}
	if num_args != method_typ.NumIn() {
		// if the in arguments number is not correct
		err := fmt.Errorf("the in arguement number is not correct")
		return &req_msg, err
	}
	for i := 0; i < num_args; i++ {
		tmp := reflect.New(method_typ.In(i))
		err := dec.DecodeValue(tmp)
		if err != nil {
			return &req_msg, err
		}
		req_msg.Args = append(req_msg.Args, tmp.Elem())
	}

	return &req_msg, nil

}

func EncodeReply(rep *ReplyMsg) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// enc.Encode(rep)
	enc.Encode(rep.Success)
	for _, item := range rep.Reply {
		enc.EncodeValue(item)
	}
	enc.Encode(rep.Err)
	new_buf, _ := ioutil.ReadAll(&buf)
	return new_buf
}

func DecodeReply(method_typ reflect.Type, dec gob.Decoder) (*ReplyMsg, error) {
	rep_msg := ReplyMsg{}
	num_args := method_typ.NumOut() - 1
	if err := dec.Decode(&rep_msg.Success); err != nil {
		log.Println(err)
	}
	for i := 0; i < num_args; i++ {
		tmp := reflect.New(method_typ.Out(i))
		err := dec.DecodeValue(tmp)
		if err != nil {
			return &rep_msg, err
		}
		rep_msg.Reply = append(rep_msg.Reply, tmp.Elem())
	}
	dec.Decode(&rep_msg.Err)
	return &rep_msg, nil
}

// send a byte-string over the socket mimicking unreliability.
// delay is emulated using time.Sleep, packet loss is emulated using RNG
// coupled with time.Sleep to emulate a timeout
func (ls *LeakySocket) SendObject(obj []byte) (bool, error) {
	if obj == nil {
		return true, nil
	}
	if ls.s != nil {
		rand.Seed(time.Now().UnixNano())
		if ls.isLossy && rand.Float32() < ls.lossRate {
			time.Sleep(time.Duration(ls.msTimeout)*time.Millisecond + time.Duration(ls.usTimeout)*time.Microsecond)
			return false, nil
		} else {
			if ls.isDelayed {
				time.Sleep(time.Duration(ls.msDelay)*time.Millisecond + time.Duration(ls.usDelay)*time.Microsecond)
			}
			_, err := ls.s.Write(obj)
			if err != nil {
				return false, errors.New("SendObject Write error: " + err.Error())
			}
			return true, nil
		}
	}
	return false, errors.New("SendObject failed, nil socket")
}

// receive a byte-string over the socket connection.
// no significant change to normal socket receive.
func (ls *LeakySocket) RecvObject() ([]byte, error) {
	if ls.s != nil {
		buf := make([]byte, 4096)
		n := 0
		var err error
		for n <= 0 {
			n, err = ls.s.Read(buf)
			if n > 0 {
				return buf[:n], nil
			}
			if err != nil {
				if err != io.EOF {
					return nil, errors.New("RecvObject Read error: " + err.Error())
				}
			}
		}
	}
	return nil, errors.New("RecvObject failed, nil socket")
}

// enable/disable emulated transmission delay and/or change the delay parameter
func (ls *LeakySocket) SetDelay(delayed bool, ms int, us int) {
	ls.isDelayed = delayed
	ls.msDelay = ms
	ls.usDelay = us
}

// change the emulated timeout period used with packet loss
func (ls *LeakySocket) SetTimeout(ms int, us int) {
	ls.msTimeout = ms
	ls.usTimeout = us
}

// enable/disable emulated packet loss and/or change the loss rate
func (ls *LeakySocket) SetLossRate(lossy bool, rate float32) {
	ls.isLossy = lossy
	ls.lossRate = rate
}

// close the socket (can also be done on original net.Conn passed to builder)
func (ls *LeakySocket) Close() error {
	return ls.s.Close()
}

// RemoteObjectError
//
// RemoteObjectError is a custom error type used for this library to identify remote methods.
// it is used by both caller and callee endpoints.
type RemoteObjectError struct {
	Err string
}

// getter for the error message included inside the custom error type
func (e *RemoteObjectError) Error() string { return e.Err }

// RequestMsg (this is only a suggestion, can be changed)
//
// RequestMsg represents the request message sent from caller to callee.
// it is used by both endpoints, and uses the reflect package to carry
// arbitrary argument types across the network.
type RequestMsg struct {
	Method string
	Args   []reflect.Value
}

// ReplyMsg (this is only a suggestion, can be changed)
//
// ReplyMsg represents the reply message sent from callee back to caller
// in response to a RequestMsg. it similarly uses reflection to carry
// arbitrary return types along with a success indicator to tell the caller
// whether the call was correctly handled by the callee. also includes
// a RemoteObjectError to specify details of any encountered failure.
type ReplyMsg struct {
	Success bool
	Reply   []reflect.Value
	Err     RemoteObjectError
}

type ServiceStatus string

const (
	Running         ServiceStatus = "RUNNING"
	Stop            ServiceStatus = "STOP"
	RunningProtocol string        = "tcp"
	HostServer      string        = "127.0.0.1:"
)

// Service -- server side stub/skeleton
//
// A Service encapsulates a multithreaded TCP server that manages a single
// remote object on a single TCP port, which is a simplification to ease management
// of remote objects and interaction with callers.  Each Service is built
// around a single struct of function declarations. All remote calls are
// handled synchronously, meaning the lifetime of a connection is that of a
// sinngle method call.  A Service can encounter a number of different issues,
// and most of them will result in sending a failure response to the caller,
// including a RemoteObjectError with suitable details.
type Service struct {
	service_typ         reflect.Type
	service_val         reflect.Value
	remote_obj_instance reflect.Value
	status              ServiceStatus
	server_addr         string
	ln                  net.Listener
	lossy               bool
	delayed             bool
	count               int
}

// build a new Service instance around a given struct of supported functions,
// a local instance of a corresponding object that supports these functions,
// and arguments to support creation and use of LeakySocket-wrapped connections.
// performs the following:
// -- returns a local error if function struct or object is nil
// -- returns a local error if any function in the struct is not a remote function
// -- if neither error, creates and populates a Service and returns a pointer
func NewService(ifc interface{}, sobj interface{}, port int, lossy bool, delayed bool) (*Service, error) {
	// if ifc is a pointer to a struct with function declarations,
	// then reflect.TypeOf(ifc).Elem() is the reflected struct's Type

	// if sobj is a pointer to an object instance, then
	// reflect.ValueOf(sobj) is the reflected object's Value

	// TODO: get the Service ready to start
	// if the function struct or object is nil, return a local error
	if ifc == nil || sobj == nil {
		return nil, fmt.Errorf("returns a local error if function struct or object is nil")
	}

	for i := 0; i < reflect.ValueOf(ifc).Elem().NumField(); i++ {
		method := reflect.ValueOf(ifc).Elem().Field(i)
		// to check if it is a remote method
		err := is_remote_func(method)
		if err != nil {
			return nil, err
		}
	}

	svc := &Service{}
	svc.service_typ = reflect.TypeOf(ifc)
	svc.service_val = reflect.ValueOf(ifc)
	svc.remote_obj_instance = reflect.ValueOf(sobj)
	svc.status = Stop
	// to get the address of the ln
	svc.server_addr = HostServer + strconv.Itoa(port)
	svc.lossy = lossy
	svc.delayed = delayed
	svc.count = 0
	return svc, nil
}

// start the Service's tcp listening connection, update the Service
// status, and start receiving caller connections
func (serv *Service) Start() error {
	if serv.status == Running {
		log.Print("Warning: This service is already running!")
	} else if serv.status == Stop {
		// otherwise, start the multithreaded tcp server at the given address
		// and update Service state
		serv.status = Running
		// TODO: start the multithreaded tcp server
		ln, err := net.Listen(RunningProtocol, serv.server_addr)
		if err != nil {
			if err.Error() == "bind: address already in use" {
			} else {
				log.Fatal(err)
			}
		}
		serv.ln = ln

		go serv.MyRun()
	}
	// IMPORTANT: Start() should not be a blocking call. once the Service
	// is started, it should return
	//
	//
	// After the Service is started (not to be done inside of this Start
	//      function, but wherever you want):
	//
	// - accept new connections from client callers until someone calls
	//   Stop on this Service, spawning a thread to handle each one
	//
	// - within each client thread, wrap the net.Conn with a LeakySocket
	//   e.g., if Service accepts a client connection `c`, create a new
	//   LeakySocket ls as `ls := LeakySocket(c, ...)`.  then:
	//
	// 1. receive a byte-string on `ls` using `ls.RecvObject()`
	//
	// 2. decoding the byte-string
	//
	// 3. check to see if the service interface's Type includes a method
	//    with the given name
	//
	// 4. invoke method
	//
	// 5. encode the reply message into a byte-string
	//
	// 6. send the byte-string using `ls.SendObject`, noting that the configuration
	//    of the LossySocket does not guarantee that this will work...
	return nil
}

/*
to start the multithreaded server in order to connect to clients
*/
func (serv *Service) MyRun() (err error) {
	for serv != nil {
		conn, err := serv.ln.Accept()

		ls := NewLeakySocket(conn, serv.lossy, serv.delayed)
		if err != nil {
			if err.Error() == fmt.Sprint("accept tcp "+serv.ln.Addr().String()+": use of closed network connection") {
			} else {
				log.Fatal(err)
			}
		} else {
			serv.count += 1
			go serv.handleRequest(ls, conn)

		}
	}
	return nil

}

func (serv *Service) GetCount() int {
	return serv.count
}

func (serv *Service) IsRunning() bool {
	if serv.status == Running {
		return true
	} else if serv.status == Stop {
		return false
	}
	return false
}

func (serv *Service) Stop() {
	if serv.status == Stop {
		return
	}
	serv.ln.Close()
	if serv.status == Running {
		serv.status = Stop
		return
	}
}

// StubFactory -- make a client-side stub
//
// StubFactory uses reflection to populate the interface functions to create the
// caller's stub interface. Only works if all functions are exported/public.
// Once created, the interface masks remote calls to a Service that hosts the
// object instance that the functions are invoked on.  The network address of the
// remote Service must be provided with the stub is created, and it may not change later.
// A call to StubFactory requires the following inputs:
// -- a struct of function declarations to act as the stub's interface/proxy
// -- the remote address of the Service as "<ip-address>:<port-number>"
// -- indicator of whether caller-to-callee channel has emulated packet loss
// -- indicator of whether caller-to-callee channel has emulated propagation delay
// performs the following:
// -- returns a local error if function struct is nil
// -- returns a local error if any function in the struct is not a remote function
// -- otherwise, uses relection to access the functions in the given struct and
//
//	populate their function definitions with the required stub functionality
func StubFactory(ifc interface{}, adr string, lossy bool, delayed bool) error {
	// if ifc is a pointer to a struct with function declarations,
	// then reflect.TypeOf(ifc).Elem() is the reflected struct's reflect.Type
	// and reflect.ValueOf(ifc).Elem() is the reflected struct's reflect.Value
	//
	// Here's what it needs to do (not strictly in this order):
	//
	//    1. create a request message populated with the method name and input
	//       arguments to send to the Service
	//
	//    2. create a []reflect.Value of correct size to hold the result to be
	//       returned back to the program
	//
	//    3. connect to the Service's tcp server, and wrap the connection in an
	//       appropriate LeakySocket using the parameters given to the StubFactory
	//
	//    4. encode the request message into a byte-string to send over the connection
	//
	//    5. send the encoded message, noting that the LeakySocket is not guaranteed
	//       to succeed depending on the given parameters
	//
	//    6. wait for a reply to be received using RecvObject, which is blocking
	//        -- if RecvObject returns an error, populate and return error output
	//
	//    7. decode the received byte-string according to the expected return types

	if ifc == nil {
		return fmt.Errorf("returns a local error if function struct or object is nil")
	}

	for i := 0; i < reflect.ValueOf(ifc).Elem().NumField(); i++ {
		method := reflect.ValueOf(ifc).Elem().Field(i)

		// to check if it is a remote method
		err := is_remote_func(method)
		if err != nil {
			return err
		}

		method_name := reflect.TypeOf(ifc).Elem().Field(i).Name

		f := func(args []reflect.Value) []reflect.Value {
			req := &RequestMsg{}
			req.Method = method_name
			for i := 0; i < len(args); i++ {
				req.Args = append(req.Args, args[i])
			}
			ls := &LeakySocket{}
			ls.isDelayed = delayed
			ls.isLossy = lossy

			conn, _ := net.Dial(RunningProtocol, adr)
			ls.s = conn
			// transport_payload, err := json.Marshal(req)
			transport_payload := EncodeRequest(req)
			// b := EncodeRequest(req)

			// new_req := DecodeRequest(b)
			// log.Println(new_req.Args)
			// log.Println(new_req.Method)
			// if err != nil {
			// 	fmt.Print("Error transfroming the json string")
			// }

			ls.SendObject(transport_payload)

			buffer, _ := ls.RecvObject()
			// message := string(buffer)

			// reply_msg := &ReplyMsg{}
			// json.Unmarshal(buffer, &reply_msg)
			// log.Println(buffer)
			dec := gob.NewDecoder(bytes.NewBuffer(buffer))
				
			reply_msg, _:= DecodeReply(method.Type(), *dec)
			// reply_msg.Err = RemoteObjectError{err.Error()}
			if !reply_msg.Success {
				log.Fatal("Failure to receive the reply message from server!")
			}
			// if there is an Remote Object error
			// in this case, it is fail to call the remote method
			if reply_msg.Err.Err != "" {
				// log.Print("Failure to call the remote method!")
				ref_args := define_method_with_err(method, reply_msg.Err.Err)
				return ref_args
			}

			// if there is no Remote Object Error
			// in this case, it is the Wrong return arg number from server
			if len(reply_msg.Reply)+1 != method.Type().NumOut() {
				// log.Print("Wrong return arg number from server")
				ref_args := define_method_with_err(method, "Wrong return arg number from server")
				return ref_args
			}

			// ref_args := make([]reflect.Value, len(reply_msg.Reply))
			// for i := 0; i < len(reply_msg.Reply); i++ {
			// 	// WARNING: not checking if can convert
			// 	ref_args[i] = reflect.ValueOf(reply_msg.Reply[i]).Convert(method.Type().Out(i))
			// 	// if reflect.ValueOf(reply_msg.Reply[i]).CanConvert(method.Type().Out(i)){

			// 	// }else{
			// 	// 	//struct

			// 	// }

			// }
			ref_args := reply_msg.Reply
			ref_args = append(ref_args, reflect.ValueOf(RemoteObjectError{}))
			return ref_args
		}
		method.Set(reflect.MakeFunc(method.Type(), f))
	}
	return nil
}

func is_remote_func(method reflect.Value) error {
	// to check if it is a remote method
	args_len := method.Type().NumOut()
	if args_len == 0 {
		return fmt.Errorf("returns a local error if any function in the struct is not a remote function")
	}
	// last_element := reflect.TypeOf(method).Out(args_len-1)
	last_element := method.Type().Out(args_len - 1)
	if last_element != reflect.TypeOf(RemoteObjectError{}) {
		// if it is not a remote method
		return fmt.Errorf("returns a local error if any function in the struct is not a remote function")
	}
	return nil
}

func define_method_with_err(method reflect.Value, err_msg string) []reflect.Value {
	num_out := method.Type().NumOut() - 1
	ref_args := make([]reflect.Value, num_out)

	for i := 0; i < num_out; i++ {
		ref_args[i] = reflect.Zero(method.Type().Out(i))
	}
	ref_args = append(ref_args, reflect.ValueOf(RemoteObjectError{err_msg}))
	return ref_args
}

func reply_err_msg(ls *LeakySocket, err_msg string) {
	// if no method is found
	reply_msg := &ReplyMsg{}
	reply_msg.Success = true
	reply_msg.Err = RemoteObjectError{err_msg}
	// transport_payload, err := json.Marshal(reply_msg)
	transport_payload := EncodeReply(reply_msg)
	// if err != nil {
	// 	fmt.Print("Error transfroming the json string")
	// }
	success, _ := ls.SendObject(transport_payload)
	for !success {
		success, _ = ls.SendObject(transport_payload)
	}
}

func (serv *Service) handleRequest(ls *LeakySocket, conn net.Conn) {
	for {
		// is_valid_flag := true
		// receive the method call from client
		buffer, _ := ls.RecvObject()
		var method_name string
		// deserialize the message sent from client
		dec := gob.NewDecoder(bytes.NewBuffer(buffer))
		if err := dec.Decode(&method_name); err != nil {
			log.Print("Unexpected error", err)
		}

		// make a new array in order to store the input args of the method
		// args := make([]reflect.Value, len(req_msg.Args))

		method := serv.remote_obj_instance.MethodByName(method_name)
		if method.IsValid() {
			req_msg, err := DecodeRequest(method_name, method.Type(), *dec)
			// if err is not empty
			if err != nil {
				reply_err_msg(ls, err.Error())
			} else {
				// reply to the client
				result := serv.remote_obj_instance.MethodByName(req_msg.Method).Call(req_msg.Args)
				reply_msg := &ReplyMsg{}
				reply_msg.Success = true
				for i := 0; i < len(result)-1; i++ {
					reply_msg.Reply = append(reply_msg.Reply, result[i])
				}
				reply_msg.Err = result[len(result)-1].Interface().(RemoteObjectError)

				// transport_payload, err := json.Marshal(reply_msg)
				transport_payload := EncodeReply(reply_msg)
				// if err != nil {
				// 	fmt.Print("Error transfroming the json string")
				// }
				success, _ := ls.SendObject(transport_payload)
				for !success {
					success, _ = ls.SendObject(transport_payload)
				}
			}
		} else {
			reply_err_msg(ls, "This method is not valid!")
		}
		// method_typ, _ := serv.service_typ.MethodByName(req_msg.Method)

	}
}
