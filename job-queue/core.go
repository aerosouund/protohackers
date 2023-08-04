package main

import (
	"encoding/json"
	"fmt"
	"jobs/types"
	"math/rand"
	"net"
	"strconv"
	"time"
)

func validateMessage(messageBytes []byte) error {
	message := string(messageBytes)
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

func handleMessage(clientExitChan chan struct{}, clientAddr string, messageBytes []byte, respCh chan map[string]any, conn net.Conn, disconnected bool) {
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
			write(conn, resp, message)
			return
		}
		if messageJson["wait"] == true {
			for {
				for _, queuename := range stringQueues {
					q := qm.Queues[queuename]
					if q != nil {
						j := q.GetJob()
						if j != nil {
							j.ClientMu.Lock()
							if j.Client == "" {
								j.Client = clientAddr
								js = append(js, j)
								j.ClientMu.Unlock()
								break
							}
						}
					}
				}
				rand.Seed(time.Now().UnixNano())
				sleepTime := rand.Intn(100)
				time.Sleep(time.Millisecond * time.Duration(sleepTime))

				if len(js) > 0 || disconnected {
					break
				}
			}
		}
		var maxPriJob = &types.Job{Priority: -1}
		for _, j := range js {
			if j.Priority > maxPriJob.Priority {
				maxPriJob = j
			}
		}

		if maxPriJob.Priority == -1 {
			resp["status"] = "no-job"
			r := types.NewResponse("ok", js[0].Queue, js[0].ID, js[0].Priority, js[0].Body)
			rstring, _ := json.Marshal(r)
			write(conn, resp, message+" "+string(rstring))
			return
		}

		// check if this client has requested jobs before or no
		// if not, initialize a map entry with their address
		qm.JobsMu.Lock()
		_, ok := qm.JobsInProgress[clientAddr]
		if !ok {
			qm.JobsInProgress[clientAddr] = make(map[string]*types.Job)
		}
		qm.JobsInProgress[clientAddr][maxPriJob.ID] = maxPriJob
		qm.JobsMu.Unlock()

		// maxPriJob.ClientMu.Lock()
		maxPriJob.Client = clientAddr
		// maxPriJob.ClientMu.Unlock()

		resp = types.NewResponse("ok", maxPriJob.Queue, maxPriJob.ID, maxPriJob.Priority, maxPriJob.Body)
		write(conn, resp, message)
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

		// q.LookupMu.Lock()
		q.JobsLookup[j.ID] = j
		// q.LookupMu.Unlock()

		resp := make(map[string]any)
		resp["id"] = j.ID
		resp["status"] = "ok"
		write(conn, resp, message)
		return

	case "abort":
		// converting the float to a string index
		id := strconv.FormatFloat(messageJson["id"].(float64), 'f', -1, 64)
		// check if this client is handling this job id
		qm.JobsMu.Lock()
		j, ok := qm.JobsInProgress[clientAddr][id]

		// remove the job from jobs in progress but don't delete it
		resp := make(map[string]any)
		resp["id"] = id
		if !ok {
			qm.JobsMu.Unlock()
			resp["status"] = "no-job"

			write(conn, resp, message)
			return
		}
		j.Client = ""
		delete(qm.JobsInProgress[clientAddr], id)
		qm.JobsMu.Unlock()
		resp["status"] = "ok"
		write(conn, resp, message)
		return

	case "delete":
		id := strconv.FormatFloat(messageJson["id"].(float64), 'f', -1, 64)
		defer qm.JobsMu.Unlock()

		resp := make(map[string]any)
		var j *types.Job = nil

		qm.JobsMu.Lock()

		for _, q := range qm.Queues {
			q.LookupMu.Lock()
			job, ok := q.JobsLookup[id]
			if ok {
				j = job
			}
			q.LookupMu.Unlock()
		}

		if j == nil || j.Deleted {
			resp["status"] = "no-job"
			write(conn, resp, message)
			return
		}

		// if this job is being handled by a client
		if j.Client != "" {
			delete(qm.JobsInProgress[j.Client], id)
		}

		// delete this job if it was found in a queue
		q := qm.Queues[j.Queue]
		// err :=
		q.DeleteJob(j.ID)
		// if err != nil {
		// 	resp["status"] = "no-job"
		// 	write(conn, resp, message)
		// 	return
		// }

		resp["id"] = id
		resp["status"] = "ok"
		write(conn, resp, message)
		return
	}
}

func aborter(clientAddr string, clientExitChan chan struct{}, disconnected *bool) {
	defer qm.JobsMu.Unlock()
	for {
		if *disconnected {
			break
		}
	}
	qm.JobsMu.Lock()
	clientJobs := qm.JobsInProgress[clientAddr]
	for _, j := range clientJobs {
		j.ClientMu.Lock()
		j.Client = ""
		j.ClientMu.Unlock()
	}
	delete(qm.JobsInProgress, clientAddr)
}

func write(conn net.Conn, r map[string]any, message string) {
	responseJson, _ := json.Marshal(r)
	fmt.Printf("responding: %v to message %v\n", string(responseJson), message)
	fmt.Fprintf(conn, string(responseJson)+"\n")
	return
}
