package main

import(
	"encoding/json"
	"log"
	"net/http"
	"net/smtp"
	"sync"

	"github.com/gorilla/websocket"
)

type Contact struct{
	Email string `json:"email`
	Phone string `json:"phone"`
}

type Message struct{
	Type string `json:"type"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

var (
	upgrader=websocket.Upgrader{
		CheckOrigin: func(r*http.Request) bool {return true},
	}
	contactStore=Contact{}
	storeLock sync.RWMutex
)

func handleUpdateContact(m Message,conn*websocket.Conn){
	if m.Email=="" && m.Phone==""{
		sendError(conn,"Atleast email or phone must be provided")
		return
	}
	storeLock.Lock()
	if m.Email!=""{
		contactStore.Email=m.Email
	}
	if m.Phone!=""{
		contactStore.Phone=m.Phone
	}
	defer storeLock.Unlock()
	log.Println("Contact Updated:",contactStore)
	sendOK(conn,"contact Information saved")
}

func handleEmergency(conn *websocket.Conn){
	storeLock.RLock()
	contact:=contactStore
	storeLock.RUnlock()
	if contact.Email=="" && contact.Phone==""{
		sendError(conn,"No contact information available")
		return
	}
	if contact.Email!=""{
		go sendEmail(contact.Email)
	}
	if contact.Phone!=""{
		go sendSMS(contact.Phone)
	}
	sendOK(conn,"Emergency alert sent")
}

func sendEmail(to string){
	from:="your-email@gmail.com"
	password:="app-password"
	msg:=[]byte("Subject: EMERGENCY ALERT\n\nThis is an emergency!")
	err:=smtp.SendMail(
		"smtp.gmail.com:587",
		smtp.PlainAuth("",from,password,"smtp.gmail.com"),
		from,
		[]string{to},
		msg,
	)
	if(err!=nil){
		log.Println("Email failed:",err)
		return
	}
	log.Println("Emergency email sent to ",to)
}

func sendSMS(phone string){
	log.Println("Emergency SMS sent to ",phone)
}

func sendOK(conn*websocket.Conn,message string){
	resp:=map[string]string{
		"status":"ok",
		"message":message,
	}
	conn.WriteJSON(resp)
}

func sendError(conn *websocket.Conn,message string){
	resp:=map[string]string{
		"status":"error",
		"message":message,
	}
	conn.WriteJSON(resp)
}

func wsHandler(w http.ResponseWriter,r*http.Request){
	conn,err:=upgrader.Upgrade(w,r,nil)
	if err!=nil{
		log.Println("Upgrade error:",err)
		return
	}
	defer conn.Close()
	log.Println("Client connected")
	for {
		_,msg,err:=conn.ReadMessage()
		if err!=nil{
			log.Println("Read error",err)
		}
		var m Message
		if err:=json.Unmarshal(msg,&m); err!=nil{
			log.Println("Invalid json:",err)
			continue
		}
		switch m.Type{
		case "update_contact":
			handleUpdateContact(m,conn)
		case "emergency":
			handleEmergency(conn)
		default:
			sendError(conn,"Unkonwn message type")
		}
	}
}

func main(){
	fs:=http.FileServer(http.Dir("./static"))
	http.Handle("/",fs)
	http.HandleFunc("/ws",wsHandler)
	log.Println("Websocket server is runniing on:8181")
	log.Fatal(http.ListenAndServe(":8181",nil))
}