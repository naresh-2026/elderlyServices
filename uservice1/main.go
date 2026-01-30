package main

import(
	"container/heap"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

//Alarm Model
type Alarm struct{
	ID string `json:"id"`
	Time time.Time `json:"time"`
	Message string `json:"message"`
}

//Priority Queue to reduce the scheduler overhead and go-routine memory
type AlarmHeap []Alarm

func(h AlarmHeap) Len() int{return len(h)}
func(h AlarmHeap) Less(i,j int) bool{return h[i].Time.Before(h[j].Time)}
func(h AlarmHeap) Swap(i,j int){h[i],h[j]=h[j],h[i]}
func(h*AlarmHeap) Push (x any){
	*h=append(*h,x.(Alarm))
}
func(h*AlarmHeap) Pop() any{
	old:=*h
	n:=len(old)
	item:=old[n-1]
	*h=old[:n-1]
	return item
}

//Scheduler 
type Scheduler struct{
	mu sync.Mutex
	heap AlarmHeap
	wake chan struct{}
}

func NewScheduler() *Scheduler{
	h:=make(AlarmHeap,0)
	heap.Init(&h)
	return &Scheduler{
		heap:h,
		wake:make(chan struct{},1),
	}
}

func(s*Scheduler) Add(alarm Alarm){
	s.mu.Lock()
	heap.Push(&s.heap,alarm)
	s.mu.Unlock()
	select{
	case s.wake<-struct{}{}:
	default:
	}
}

func(s*Scheduler) Run(){
	for{
		s.mu.Lock()

		if len(s.heap)==0{
			s.mu.Unlock()
			<-s.wake
			continue
		}

		next:=s.heap[0]
		wait:=time.Until(next.Time)
		s.mu.Unlock()
		if wait>0{
			select{
			case<-time.After(wait):
			case<-s.wake:
				continue
			}
		}

		s.mu.Lock()
		alarm:=heap.Pop(&s.heap).(Alarm)
		s.mu.Unlock()

		notifyClints(alarm)
	}
}

//Websocket Handling
var(
	upgrader=websocket.Upgrader{
		CheckOrigin:func(r*http.Request) bool{
			return true
		},
	}
	clients=make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

func wsHandler(w http.ResponseWriter,r*http.Request){
	conn,err:=upgrader.Upgrade(w,r,nil)
	if err!=nil{
		return
	}

	clientsMu.Lock()
	clients[conn]=true
	clientsMu.Unlock()
}

func notifyClints(alarm Alarm){
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for c:=range clients{
		err:=c.WriteJSON(alarm)
		if err!=nil{
			c.Close()
			delete(clients,c)
		}
	}
}

//HTTP API
var scheduler=NewScheduler()

func createAlarmHandler(w http.ResponseWriter,r*http.Request){
	if r.Method!=http.MethodPost{
		http.Error(w,"Method is not allowed",http.StatusMethodNotAllowed)
		return
	}
	var req struct{
		Time string `json:"time"`
		Message string `json:"message"`
	}
	if err:=json.NewDecoder(r.Body).Decode(&req); err!=nil{
		http.Error(w,"Invalid body",http.StatusBadRequest)
		return
	}
	t,err:=time.Parse(time.RFC3339,req.Time)
	if err!=nil{
		http.Error(w,"Invalid time format",http.StatusBadRequest)
		return
	}

	alarm:=Alarm{
		ID: uuid.NewString(),
		Time: t,
		Message: req.Message,
	}
	scheduler.Add(alarm)
	w.WriteHeader(http.StatusCreated)
}

//Main function
func main(){
	go scheduler.Run()

	http.Handle("/",http.FileServer(http.Dir("./static")))
    http.HandleFunc("/alarms",createAlarmHandler)
	http.HandleFunc("/ws",wsHandler)
	log.Println("Alarm service running on port:8180")
	log.Fatal(http.ListenAndServe(":8180",nil))
}