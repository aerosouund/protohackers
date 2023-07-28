package main

import (
	"context"
	"encoding/json"
	"fmt"
	"jobs/types"
	"net"
	"strconv"
)

func validateMessage(messageBytes []byte) error {
	message := string(messageBytes)
	fmt.Println("message: ", message)
	var messageInterface interface{}

	err := json.Unmarshal([]byte(message), &messageInterface)
	if err != nil {
		return types.ErrInvalidMessage
	}

	messageJson, ok := messageInterface.(map[string]any)
	if !ok {
		return types.ErrInvalidMessage
	}
	r, _ := messageJson["request"].(string)

	switch r {
	case "get":
		requiredKeys := []string{"queues"}
		for _, key := range requiredKeys {
			if _, ok := messageJson[key]; !ok {
				return types.ErrInvalidMessage
			}
		}
		return nil
	case "put":
		requiredKeys := []string{"pri", "queue", "job"}
		for _, key := range requiredKeys {
			if _, ok := messageJson[key]; !ok {
				return types.ErrInvalidMessage
			}
		}
		return nil
	case "abort":
		_, ok := messageJson["id"]
		if !ok {
			return types.ErrInvalidMessage
		}
		return nil
	case "delete":
		_, ok := messageJson["id"]
		if !ok {
			return types.ErrInvalidMessage
		}
		return nil
	default:
		return types.ErrInvalidMessage
	}
}

func handleMessage(clientExitChan chan struct{}, clientAddr string, messageBytes []byte, respCh chan map[string]any, conn net.Conn) {
	message := string(messageBytes)
	var messageJson map[string]any

	json.Unmarshal([]byte(message), &messageJson)
	switch messageJson["request"] {
	case "get":
		var js []*types.Job
		queues := messageJson["queues"].([]any)

		stringQueues := make([]string, len(queues)) // convert array of queues from arr of any to arr of string
		for i, v := range queues {
			stringQueues[i] = v.(string)
		}
		for _, queueName := range stringQueues {

			q, ok := qm.Queues[queueName]
			if ok {
				j := q.GetJob()

				if j != nil && j.Client == "" {
					js = append(js, j)
				}
			}
		}
		// get jobs from all queues and return max pri
		resp := make(map[string]any)
		if len(js) == 0 && messageJson["wait"] != true {
			resp["status"] = "no-job"
			write(conn, resp)
			return
		}
		if len(js) == 0 && messageJson["wait"] == true {
			j := waitOnJobs(clientExitChan, stringQueues)
			if j == nil {
				return
			}
			js = append(js, j)
		}
		var maxPriJob = &types.Job{Priority: 0}
		for _, j := range js {
			if j.Priority > maxPriJob.Priority {
				maxPriJob = j
			}
		}

		// check if this client has requested jobs before or no
		// if not, initialize a map entry with their address
		_, ok := qm.JobsInProgress[clientAddr]
		if !ok {
			qm.JobsInProgress[clientAddr] = make(map[string]*types.Job)
		}

		qm.JobsMu.Lock()
		qm.JobsInProgress[clientAddr][maxPriJob.ID] = maxPriJob
		qm.JobsMu.Unlock()

		maxPriJob.ClientMu.Lock()
		maxPriJob.Client = clientAddr
		maxPriJob.ClientMu.Unlock()

		resp = types.NewResponse("ok", maxPriJob.Queue, maxPriJob.ID, maxPriJob.Priority, maxPriJob.Body)
		write(conn, resp)
		return

	case "put":
		j := types.NewJob(int(messageJson["pri"].(float64)), messageJson["job"].(map[string]interface{}), messageJson["queue"].(string))
		q, ok := qm.Queues[messageJson["queue"].(string)]
		if !ok {
			qname := messageJson["queue"].(string)
			q = types.NewQueue()
			qm.Queues[qname] = q
		}

		q.PutJob(j)

		q.LookupMu.Lock()
		q.JobsLookup[j.ID] = j
		q.LookupMu.Unlock()

		resp := make(map[string]any)
		resp["id"] = j.ID
		resp["status"] = "ok"
		write(conn, resp)
		return

	case "abort":
		// converting the float to a string index
		id := strconv.FormatFloat(messageJson["id"].(float64), 'f', -1, 64)
		// check if this client is handling this job id
		j, ok := qm.JobsInProgress[clientAddr][id]

		// remove the job from jobs in progress but don't delete it
		resp := make(map[string]any)
		resp["id"] = id
		if !ok {
			resp["status"] = "no-job"
			write(conn, resp)
			return
		}
		j.Client = ""
		resp["status"] = "ok"
		write(conn, resp)
		return

	case "delete":
		id := strconv.FormatFloat(messageJson["id"].(float64), 'f', -1, 64)

		resp := make(map[string]any)
		var j *types.Job = nil

		for _, q := range qm.Queues {
			job, ok := q.JobsLookup[id]
			if ok {
				j = job
			}
		}

		if j == nil {
			resp["status"] = "no-job"
			write(conn, resp)
			return
		}

		// if this job is being handled by a client
		if j.Client != "" {
			delete(qm.JobsInProgress[j.Client], id)
		}

		// delete this job if it was found in a queue
		q := qm.Queues[j.Queue]
		err := q.DeleteJob(j.ID)
		if err != nil {
			resp["status"] = "no-job"
			write(conn, resp)
			return
		}

		resp["id"] = id
		resp["status"] = "ok"
		write(conn, resp)
		return
	}
}

func waitOnJobs(clientExitChan chan struct{}, queues []string) *types.Job {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobCh := make(chan *types.Job)
	for _, queuename := range queues {
		go waitOnQueue(ctx, queuename, jobCh, clientExitChan)
	}

	for {
		select {
		case <-clientExitChan:
			// fmt.Println("canceling goroutines")
			// cancel()
			return nil
		case j := <-jobCh:
			return j
		}
	}
}

func waitOnQueue(ctx context.Context, queueName string, jobCh chan<- *types.Job, clientExitChan <-chan struct{}) {
	// prevent goroutines from leaking if the client cancelled
	fmt.Println("starting wait on jobs")
	for {
		select {
		case <-clientExitChan:
			fmt.Println("canceling goroutines")
			return
		case <-ctx.Done():
			fmt.Println("goroutine exiting")
			return
		default:
			q, ok := qm.Queues[queueName]
			if !ok {
				fmt.Println("queue doesnt exist")
				return
			}
			if ok {
				j := q.GetJob()
				if j != nil {
					resp := types.NewResponse("ok", j.Queue, j.ID, j.Priority, j.Body)
					write(conn, resp)
					return
				}
			}
		}
	}
}

func aborter(clientAddr string, clientExitChan chan struct{}) {
	<-clientExitChan
	clientJobs := qm.JobsInProgress[clientAddr]
	for _, j := range clientJobs {
		j.ClientMu.Lock()
		j.Client = ""
		j.ClientMu.Unlock()
	}
	delete(qm.JobsInProgress, clientAddr)
}

func write(conn net.Conn, r map[string]any) {
	responseJson, _ := json.Marshal(r)
	fmt.Println("responding:", string(responseJson))
	fmt.Fprintf(conn, string(responseJson)+"\n")
}
