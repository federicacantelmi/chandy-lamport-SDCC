package utils

type NodeChannels struct {
	SnapshotProcessChannel chan Command // chan per comunicare dal server RPC al process di inviare messaggio
	ProcessChannel         chan Command // chan per comunicare dal server RPC allo snapProcess di iniziare snapshot
}

type Command struct {
	Name         string      // nome del comando da eseguire ("StartSNapshot", "SendMessage")
	Payload      interface{} // dati in più -> receiver, content, ...
	ReplyChannel chan error  // Canale su cui ricevo l'esito del comando
}

func CreateNodeChannels(size int) NodeChannels {
	return NodeChannels{
		SnapshotProcessChannel: make(chan Command, size),
		ProcessChannel:         make(chan Command, size),
	}
}
