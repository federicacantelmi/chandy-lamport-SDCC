package snapshot

import (
	"chandy-lamport/src/process"
	"chandy-lamport/src/utils"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type SnapshotFile struct {
	Timestamp    time.Time           `json:"timestamp"`
	SnapshotID   int                 `json:"snapshot_id"`
	ProcessID    int                 `json:"process_id"`
	InternState  int                 `json:"intern_state"`
	ChannelState map[int]interface{} `json:"channel_state"`
}

var (
	snapshotList      []*Snapshot
	snapshotListMutex sync.Mutex
)

const (
	ChannelStatusDone  = "done"
	ChannelStatusEmpty = "empty"
	ChannelStatusGoing = "going"
)

type Process interface {
	GetProcessID() int
	GetTCPAddress() *net.TCPAddr
}

type ChannelState struct {
	Messages []int  `json:"messages"` // Lista dei messaggi ricevuti su un canale dopo che è stato avviato uno snapshot
	Status   string `json:"status"`   // Stato del canale, può essere "done" se ha ricevuto marker su quel canale per quello snapshot, "empty" se non ha ricevuto messaggi, "going" se sta ancora registrando messaggi in ingresso
}

type Snapshot struct {
	Timestamp            time.Time
	ProcessId            int                   // Id del processo
	SnapshotId           int                   // Id dello snapshot, ovvero del processo che ha iniziato lo snapshot
	balance              int                   // Balance interno dello snapshot process
	inputChannelStates   map[int]*ChannelState // Mappa che associa all'id di ogni altro processo, un ChannelState
	receivedMarkers      map[int]bool          // Mappa che associa all'id di ogni altro processo un booleano per capire se ha ricevuto o meno il marker da quel processo
	channelsStateMutex   sync.Map              // Mutex che sincronizza l'accesso ad ogni canale
	receivedMarkersMutex sync.Mutex            // Mutex che sincronizza l'accesso alla mappa receivedMarkers
	replyChannel         chan error
}

type SnapshotProcess struct {
	Process              *process.ProcessStruct // Riferimento al processo
	channels             utils.NodeChannels     // Canali utilizzati per comunicare con le altre due parti
	receivedMarkersMutex sync.Mutex             // Mutex per gestire i marker ricevuti
}

// Funzione che crea uno SnapshotProcess
func CreateSnapshotProcess(process *process.ProcessStruct, channels utils.NodeChannels) *SnapshotProcess {
	snapshotProcess := SnapshotProcess{
		Process:  process,
		channels: channels,
	}
	fmt.Printf("Snapshot Process %d created succesfully\n", process.ProcessID)
	go snapshotProcess.listen()
	return &snapshotProcess
}

// Funzione che rimane in ascolto sui canali per ricevere comandi da eseguire
func (sp *SnapshotProcess) listen() {
	for {
		select {
		case cmd := <-sp.channels.SnapshotProcessChannel:
			switch cmd.Name {
			case "StartSnapshot":
				go sp.startSnapshot(cmd.ReplyChannel)
			case "MessageReceived":
				if msg, ok := cmd.Payload.(utils.Message); ok {
					// fmt.Printf("SnapshotProcess %d: received message from process %d: %d\n", msg.Receiver, msg.Sender, msg.Content)
					handleMessageReceived(msg)
				}
			case "MarkerReceived":
				if msg, ok := cmd.Payload.(utils.Message); ok {
					// fmt.Printf("Process %d: Sending message to process %d: %d\n", msg.Sender, msg.Receiver, msg.Content)
					go sp.handleMarker(msg, sp.Process)
				}
			default:
				log.Printf("Process %d: Unknown command %s.\n", sp.Process.GetProcessID(), cmd.Name)
			}
		}
	}
}

func (sp *SnapshotProcess) createSnapshot(processId int, snapshotId int, localState int, channelState map[int]*ChannelState, receivedMarkers map[int]bool, repChannel chan error) *Snapshot {
	return &Snapshot{
		Timestamp:          time.Now(),
		ProcessId:          processId,
		SnapshotId:         snapshotId,
		balance:            localState,
		inputChannelStates: channelState,
		receivedMarkers:    receivedMarkers,
		channelsStateMutex: sync.Map{},
		replyChannel:       repChannel,
	}
}

func (sp *SnapshotProcess) startSnapshot(replyChannel chan error) {

	sp.channels.ProcessChannel <- utils.Command{Name: "StartingSnapshot", Payload: nil}
	channelState := make(map[int]*ChannelState)

	receivedMarkers := make(map[int]bool)

	// Imposto tutti i bool a false tranne quello relativo al processo da cui ho appena ricevuto il marker
	for i := 0; i < len(utils.ProcessList); i++ {
		receivedMarkers[i+1] = false
	}
	receivedMarkers[sp.Process.GetProcessID()] = true

	// Creo un'istanza di Snapshot relativa allo snapshot che sto avviando
	snapshot := sp.createSnapshot(sp.Process.GetProcessID(), sp.Process.GetProcessID(), sp.Process.GetBalance(), channelState, receivedMarkers, replyChannel)

	// Aggiungo l'istanza di Snapshot alla lista degli snapshot attivi per questo processo
	snapshotListMutex.Lock()
	snapshotList = append(snapshotList, snapshot)
	snapshotListMutex.Unlock()

	fmt.Printf("Process %d: started snapshot\n", sp.Process.GetProcessID())

	// Chiedo a Process di inviare i markers agli altri processi
	sp.channels.ProcessChannel <- utils.Command{Name: "SendMarkers", Payload: utils.Message{Sender: sp.Process.GetProcessID(), SnapshotID: sp.Process.GetProcessID()}}
}

// Funzione che salva uno Snapshot per il processo quando va terminato
func saveSnapshot(snapshot *Snapshot) error {
	filename := "snapshot_" + strconv.Itoa(snapshot.SnapshotId) + "_process_" + strconv.Itoa(snapshot.ProcessId) + ".json"
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Error creating snapshot file: %s\n", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	snapshotFile := SnapshotFile{
		Timestamp:    snapshot.Timestamp,
		SnapshotID:   snapshot.SnapshotId,
		ProcessID:    snapshot.ProcessId,
		InternState:  snapshot.balance,
		ChannelState: make(map[int]interface{}),
	}

	// Se ho ricevuto messaggi, salvo la lista dei messaggi; se non ho ricevuto messaggi, salvo la stringa "empty"
	for sender, channelState := range snapshot.inputChannelStates {
		if channelState == nil || len(channelState.Messages) == 0 {
			snapshotFile.ChannelState[sender] = ChannelStatusEmpty
		} else {
			snapshotFile.ChannelState[sender] = channelState.Messages
		}
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(snapshotFile); err != nil {
		return fmt.Errorf("Error encoding snapshot file: %s\n", err)
	}
	fmt.Printf("Terminated and saved snapshot to file: %s\n", filename)
	return nil
}

// Funzione che esegue le operazioni finali di uno snapshot
func terminateLocalSnapshot(snapshot *Snapshot) {
	if err := saveSnapshot(snapshot); err != nil {
		if snapshot.replyChannel != nil {
			snapshot.replyChannel <- err
		} else {
			log.Printf("Error saving snapshot: %v", err)
		}
	}
	if snapshot.replyChannel != nil {
		snapshot.replyChannel <- nil
	}

	snapshotListMutex.Lock()
	// Elimino snapshot dalla snapshot list
	for i, snap := range snapshotList {
		if snap.SnapshotId == snapshot.SnapshotId {
			snapshotList = append(snapshotList[:i], snapshotList[i+1:]...)
			break
		}
	}
	snapshotListMutex.Unlock()
}

// Funzione che gestisce l'arrivo di un messaggio al nodo
func handleMessageReceived(message utils.Message) {
	snapshotListMutex.Lock()

	// Se non c'è istanza attiva di snapshot, non devo salvare il messaggio nello stato di nessun canale
	if snapshotList == nil {
		snapshotListMutex.Unlock()
		return
	}

	// Se esiste qualche istanza di snapshot attiva, devo controllare se il canale da cui ricevo è "going" oppure "done" o "empty"
	for i, snapshot := range snapshotList {
		var channelStateMutex *sync.Mutex
		// Carico il mutex associato al sender del messaggio, se non esiste, ne creo uno nuovo
		loadedMutex, _ := snapshot.channelsStateMutex.LoadOrStore(message.Sender, &sync.Mutex{})

		// Type assertion così il mutex è di tipo *sync.Mutex
		channelStateMutex = loadedMutex.(*sync.Mutex)

		channelStateMutex.Lock()

		// Prelevo (se esiste) stato del canale con il processo sender relativo allo snapshot su cui sto iterando
		chanState, exists := snapshot.inputChannelStates[message.Sender]
		if !exists {
			// Se non esiste stato del canale -> lo creo perché vuol dire che è il primo messaggio che ricevo
			chanState = &ChannelState{
				Messages: make([]int, 0),
				Status:   ChannelStatusGoing,
			}

			// Aggiungo il messaggio alla lista di messaggi dello stato del canale con il processo sender relativo allo snapshot su cui sto iterando
			chanState.Messages = append(chanState.Messages, message.Content)
			snapshot.inputChannelStates[message.Sender] = chanState
			fmt.Printf("Process %d: created channelState for first message: %v\n", snapshot.ProcessId, snapshot.inputChannelStates[message.Sender].Messages)
		} else {
			// Se esiste stato del canale e vale "going" (quindi devo ancora ricevere marker per questo snapshot da quel processo), aggiungo il messaggio
			if chanState.Status == ChannelStatusGoing {
				chanState.Messages = append(chanState.Messages, message.Content)
				fmt.Printf("Process %d: added message to channel state %d for snapshot %d. Now channelState messages are: %v\n", snapshot.ProcessId, message.Sender, snapshot.SnapshotId, chanState.Messages)
				snapshot.inputChannelStates[message.Sender] = chanState
			} else {
				// Se lo stato del canale esiste ma non è going significa che ho già ricevuto il marker per questo snapshot -> non devo aggiungere il messaggio allo stato del canale
				continue
			}
		}
		snapshotList[i] = snapshot
		channelStateMutex.Unlock()
	}
	snapshotListMutex.Unlock()
}

// Funzione che gestisce l'arrivo di un marker
func (sp *SnapshotProcess) handleMarker(marker utils.Message, p *process.ProcessStruct) {
	active := false
	var newSnapshot *Snapshot

	snapshotListMutex.Lock()

	// Itero su snapshotList per verificare se esiste già un'istanza di SnapshotProcess attiva relativa allo snapshotID appena ricevuto nel marker
	for _, snap := range snapshotList {
		if snap.SnapshotId == marker.SnapshotID {
			fmt.Printf("ProcessStruct %d: already existing istance for snapshot %d\n", p.GetProcessID(), marker.SnapshotID)
			active = true
			newSnapshot = snap
			continue
		}
	}

	// Se non esiste un'istanza di SnapProcess attiva relativa a questo snapshot -> la creo
	if !active {

		fmt.Printf("ProcessStruct %d: received marker from %d for snapshot %d but snapshot not active\n", p.GetProcessID(), marker.Sender, marker.SnapshotID)

		// Creo la lista di received markers relativa a questo snapshot
		sp.receivedMarkersMutex.Lock()
		receivedMarkers := make(map[int]bool)
		for i := 0; i < len(utils.ProcessList); i++ {
			receivedMarkers[i+1] = false
		}
		receivedMarkers[p.GetProcessID()] = true
		sp.receivedMarkersMutex.Unlock()

		// Creo lo snapshot
		sp.Process.InternStateMutex.Lock()
		snapshot := sp.createSnapshot(p.GetProcessID(), marker.SnapshotID, sp.Process.Balance, make(map[int]*ChannelState), receivedMarkers, nil)
		sp.Process.InternStateMutex.Unlock()

		// Aggiungo alla snapshotList lo snapshot appena creato
		snapshotList = append(snapshotList, snapshot)

		// Siccome è il primo marker relativo a questo snapshot, invio marker agli altri processi
		sp.channels.ProcessChannel <- utils.Command{Name: "SendMarkers", Payload: utils.Message{Sender: snapshot.ProcessId, SnapshotID: snapshot.SnapshotId}}
		newSnapshot = snapshot
	} else {
		sp.channels.ProcessChannel <- utils.Command{Name: "MutexRelease", Payload: utils.Message{}}
	}

	snapshotListMutex.Unlock()
	if newSnapshot == nil {
		log.Println("Error: snapProcess is nil")
		return
	}

	// Ho ricevuto il marker dal processo con ID marker.Sender => registro questo fatto
	sp.receivedMarkersMutex.Lock()
	newSnapshot.receivedMarkers[marker.Sender] = true
	sp.receivedMarkersMutex.Unlock()

	var channelStateMutex *sync.Mutex
	// Carico il mutex associato al sender del messaggio, se non esiste, ne creo uno nuovo
	loadedMutex, _ := newSnapshot.channelsStateMutex.LoadOrStore(marker.Sender, &sync.Mutex{})
	// Type assertion così il mutex è di tipo *sync.Mutex
	channelStateMutex = loadedMutex.(*sync.Mutex)
	channelStateMutex.Lock()

	// Se non esiste chanState significa che non ho ricevuto messaggi da quel canale dopo che è stato avviato lo snapshot, altrimenti l'avrei creata per aggiungerci i messaggi ricevuti
	chanState, exists := newSnapshot.inputChannelStates[marker.Sender]
	if !exists {
		chanState = &ChannelState{
			Messages: make([]int, 0),
			Status:   ChannelStatusEmpty,
		}
		fmt.Printf("Process %d: marking channel %d as empty for snapshot %d\n", newSnapshot.ProcessId, marker.Sender, marker.SnapshotID)
	} else {
		chanState.Status = ChannelStatusDone
		fmt.Printf("Process %d: marking channel %d as done for snapshot %d\n", newSnapshot.ProcessId, marker.Sender, marker.SnapshotID)
	}

	newSnapshot.inputChannelStates[marker.Sender] = chanState
	channelStateMutex.Unlock()

	// Controllo se ho ricevuto tutti i marker
	allMarkersReceived := true
	sp.receivedMarkersMutex.Lock()
	for _, received := range newSnapshot.receivedMarkers {
		if !received {
			allMarkersReceived = false
			break
		}
	}
	sp.receivedMarkersMutex.Unlock()
	// Se quello appena ricevuto è l'ultimo marker che mi aspettavo
	if allMarkersReceived {
		fmt.Printf("ProcessStruct %d: received all markers for snapshot %d\n", newSnapshot.ProcessId, marker.SnapshotID)
		terminateLocalSnapshot(newSnapshot)
	}
}
