# Progetto SDCC: Implementazione dell'algoritmo di Chandy-Lamport per la creazione di uno snapshot globale

### Esecuzione su istanza EC2
Dopo aver avviato un'istanza EC2 dalla dashboard di AWS sarà necessario collegarvisi via SSH: eseguire il comando `ssh -i <private-key>.pem ec2-user@<VM-Public-IPv4>`, inserendo come chiave privata quella ottenuta in fase di setup dell'istanza EC2.

Successivamente:
1. Installazione delle dipendenze:
   1. installare docker: `sudo yum install docker -y`
   2. installare docker-compose: `sudo curl -L https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m) -o /usr/local/bin/docker-compose`
   3. eseguire docker-compose: `sudo chmod +x /usr/local/bin/docker-compose`
   4. `sudo yum install git -y`
2. Clone della repository del progetto:
   1. `git clone https://github.com/federicacantelmi/chandy-lamport-SDCC.git`
   2. `cd chandy-lamport-SDCC`
3. Lancio del progetto con Docker Compose:
   1. avviare il servizio di docker: `sudo service docker start`
   2. eseguo il file: `sudo bash ./run_and_check`

A questo punto nel terminale verranno mostrati:
1. La somma dei bilanci iniziali dei dei processi.
2. Le fasi di building dei container per i processi ed il client.
3. Il bilancio totale ottenuto in ogni snapshot.
4. Messaggio di successo o errore nel caso in cui il bilancio iniziale totale ed il bilancio finale totale corrispondano o meno.

Se si vogliono osservare gli output di uno specifico container si può eseguire il comando `sudo docker logs -f <container-id>`, dove <container-id> sarà della forma:
- `process<numero del processo>`, nel caso del container di un processo.
- `client`, nel caso del container in cui esegue il client.

Se si vuole pulire del tutto l'ambiente di esecuzione si può eseguire il comando `sudo docker rm $(sudo docker ps -a -q)`
per eliminare tutti i container.

Per arrestare del tutto Docker: `sudo service docker stop 2> /dev/null && sudo systemctl stop docker.socket`.

Se si vuole eliminare tutta la cache di Docker: `sudo docker builder prune -a`.


### Environment
- `NUM_PROCESS`: Numero di processi che si vogliono attivare nel sistema.
- `TCP_PORT`: Porta su cui ogni processo rimane in ascolto per ricevere messaggi da parte di altri processi.
- `RPC_PORT`: Porta su cui il server RPC rimane in ascolto per l'invocazione dei metodi da parte del client RPC.
- `MAX_RETRIES`: Numero massimo di volte per cui un processo prova a connettersi ad un altro processo per l'invio di un messaggio.
- `INITIAL_BALANCE_PROCESS<ID_PROCESS>`: Bilancio iniziale del processo di cui si specifica l'id.
