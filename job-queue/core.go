package main

import (
	"encoding/json"
	"fmt"
	"jobs/types"
	"net"
	"strconv"
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

func handleMessage(clientAddr string, messageBytes []byte, conn net.Conn, disconnected bool) {
	message := string(messageBytes)
	var messageJson map[string]any

	json.Unmarshal([]byte(message), &messageJson)
	switch messageJson["request"] {
	case "get":
		queues := messageJson["queues"].([]any)

		stringQueues := make([]string, len(queues)) // convert array of queues from arr of any to arr of string
		for i, v := range queues {
			stringQueues[i] = v.(string)
		}
		var j = &types.Job{}

		if messageJson["wait"] != true {
			j = qm.GetJob(clientAddr, stringQueues)
			if j == nil {
				resp := make(map[string]any)
				resp["status"] = "no-job"
				write(conn, resp, message)
				return
			}
			resp := types.NewResponse("ok", j.Queue, j.ID, -j.Priority, j.Body)
			write(conn, resp, message)

			qm.Lock()
			qm.JobsInProgress[clientAddr] = append(qm.JobsInProgress[clientAddr], j)
			qm.Unlock()

			return
		}
		// get jobs from all queues and return max pri
		if messageJson["wait"] == true {
			for {
				j = qm.GetJob(clientAddr, stringQueues)
				if j != nil || disconnected {
					break
				}
			}
			if disconnected {
				return
			}
			resp := types.NewResponse("ok", j.Queue, j.ID, -j.Priority, j.Body)
			write(conn, resp, message)
			qm.JobsInProgress[clientAddr] = append(qm.JobsInProgress[clientAddr], j)

			return

		}

	case "put":
		j := types.NewJob(int(messageJson["pri"].(float64)), messageJson["job"].(map[string]interface{}), messageJson["queue"].(string))
		q, ok := qm.Queues[messageJson["queue"].(string)]
		if !ok {
			qm.Lock()
			qname := messageJson["queue"].(string)
			q = types.NewQueue()
			qm.Queues[qname] = q
			qm.Unlock()
		}

		qm.PutJob(q, j)

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
		found := false
		var j = &types.Job{}
		for _, job := range qm.JobsInProgress[clientAddr] {
			if job.ID == id && !job.Deleted {
				found = true
				j = job
			}
		}
		qm.JobsMu.Unlock()

		// remove the job from jobs in progress but don't delete it
		resp := make(map[string]any)
		resp["id"] = id
		if !found {
			resp["status"] = "no-job"
			write(conn, resp, message)
			return
		}
		j.Priority = -j.Priority
		// qm.JobsInProgress[clientAddr] = append(qm.JobsInProgress[clientAddr], ) this means several clients can have
		// the job in their arrays, may have unintended effects
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
			q.LookupMu.Lock() // may delete
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
			// delete(qm.JobsInProgress[j.Client], id)
			j.Client = ""
		}

		// delete this job if it was found in a queue
		qm.DeleteJob(j)

		resp["id"] = id
		resp["status"] = "ok"
		write(conn, resp, message)
		return
	}
}

func abort(clientAddr string) {
	defer qm.JobsMu.Unlock()
	qm.JobsMu.Lock()
	clientJobs := qm.JobsInProgress[clientAddr]
	for _, j := range clientJobs {
		j.Priority = -j.Priority
	}
	delete(qm.JobsInProgress, clientAddr)
}

func write(conn net.Conn, r map[string]any, message string) {
	responseJson, _ := json.Marshal(r)
	fmt.Printf("responding: %v to message %v\n", string(responseJson), message)
	fmt.Fprintf(conn, string(responseJson))
	return
}
