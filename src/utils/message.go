package utils

type MessageType string

const (
	MESSAGE MessageType = "MESSAGE"
	MARKER  MessageType = "MARKER"
)

// todo aggiungi snapshot
type Message struct {
	Type       MessageType `json:"Type"`       // Tipo del messaggio: vale 0 se è un message, vale 1 se è un marker
	Sender     int         `json:"Sender"`     // Id del processo sender
	Receiver   int         `json:"Receiver"`   // Id del processo receiver
	SnapshotID int         `json:"SnapshotId"` // Id dello snapshot
	Content    int         `json:"Content"`    // Quantità da aggiungere o rimuovere dal bilancio
}
