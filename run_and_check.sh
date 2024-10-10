#!/bin/bash

# File .env che contiene i bilanci iniziali
ENV_FILE=".env"

# Controllo se il file .env esiste
if [ ! -f "$ENV_FILE" ]; then
    echo "Errore: Il file .env non esiste."
    exit 1
fi

if [ -d "snapshots" ]; then
    rm -rf snapshots
fi

# Estraggo i bilanci iniziali
initial_balance=0
while IFS='=' read -r key value || [ -n "$key" ]; do
    # Considera solo le righe che iniziano con "PROCESS_BALANCE"
    if [[ $key == INITIAL_BALANCE_PROCESS* ]]; then
        # Somma i bilanci iniziali dei processi (rimuove eventuali spazi attorno al valore)
        clean_value=$(echo "$value" | xargs)
        initial_balance=$(echo "$initial_balance + $clean_value" | bc)
    fi
done < "$ENV_FILE"

echo "Bilancio iniziale totale: $initial_balance"

# Avvio i container Docker specificati nel docker-compose file
docker compose down
docker-compose up -d --build

# Controllo se l'avvio è riuscito
if [ $? -ne 0 ]; then
    echo "Errore: Avvio dei container fallito."
    exit 1
fi

echo "Container avviati con successo."

# Attendo che il client termini
CLIENT_CONTAINER="client"

echo "In attesa che il container '$CLIENT_CONTAINER' termini..."
docker wait "$CLIENT_CONTAINER"

# Prelevo i file dai container dei processi
PROCESS_CONTAINERS=("process1" "process2" "process3")

# Creo una directory per salvare gli snapshot
SNAPSHOT_DIR="snapshots"
mkdir -p "$SNAPSHOT_DIR"

# Copio i file snapshot da ogni container di processo alla directory locale
for container in "${PROCESS_CONTAINERS[@]}"; do
    echo "Copia degli snapshot dal container '$container'..."
    docker cp "$container:/app/." "$SNAPSHOT_DIR/$container"
done

# Eseguo la somma dei valori degli snapshot
declare -A snapshot_sums

# Itero su ogni file snapshot per ogni container
for snapshot_file in $(find "$SNAPSHOT_DIR" -type f -name "snapshot_*"); do
  snapshot_id=$(basename "$snapshot_file" | grep -oP '(?<=snapshot_)\d+(?=_process)')
    # Verifica che lo snapshot_id sia valido
    if [[ -z "$snapshot_id" ]]; then
        echo "Errore: impossibile determinare lo snapshot ID dal file $snapshot_file."
        continue
    fi

    echo "Analizzando il file: $snapshot_file con snapshot ID: $snapshot_id"
    cat $snapshot_file

    # Estrazione del valore di InternalState e ChannelState
    internal_state=$(jq '.intern_state' "$snapshot_file")

    channel_state=$(jq '[.channel_state[] | select(type == "array") | .[]] | add' "$snapshot_file")

    # Verifica e somma dei valori estratti
    if [[ -n "$internal_state" && -n "$channel_state" ]]; then
        snapshot_sums["$snapshot_id"]=$((snapshot_sums["$snapshot_id"] + internal_state + channel_state))
    else
        echo "Errore: Non sono riuscito a estrarre InternalState o ChannelState dal file $snapshot_file."
    fi
done

# Confronto i valori ottenuti con il bilancio iniziale

for snapshot_id in "${!snapshot_sums[@]}"; do
    # Calcolo il bilancio totale dello snapshot corrente (stato interno + canali)
    total_snapshot_balance=$(echo "${snapshot_sums[$snapshot_id]}" | bc)

    echo "Bilancio totale per Snapshot ID $snapshot_id: $total_snapshot_balance"

    # Confronto con il bilancio iniziale
    if [ "$(echo "$total_snapshot_balance == $initial_balance" | bc)" -eq 1 ]; then
        echo "Successo: Il bilancio totale dello Snapshot con ID $snapshot_id corrisponde al bilancio iniziale."
    else
        echo "Errore: Il bilancio totale dello Snapshot con ID $snapshot_id ($total_snapshot_balance) non corrisponde al bilancio iniziale ($initial_balance)."
    fi
done
