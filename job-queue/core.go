package main

import (
	"encoding/json"
	"io"
	"jobs/types"
	"net"
	"sort"
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
		if len(messageJson["queues"].([]any)) == 0 {
			return types.ErrInvalidMessage
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
			resp := types.NewResponse("ok", j.Queue, j.ID, j.Priority, j.Body)
			qm.Lock()
			qm.JobsInProgress[clientAddr] = append(qm.JobsInProgress[clientAddr], j)
			qm.Unlock()
			write(conn, resp, message)
			return
		}
		// get jobs from the first available queue
		if messageJson["wait"] == true {
			for {
				time.Sleep(time.Millisecond * 100)
				j = qm.GetJob(clientAddr, stringQueues)
				// job is found
				if j != nil {
					break
				}
				// client ended connection
				if disconnected {
					return
				}
			}

			resp := types.NewResponse("ok", j.Queue, j.ID, j.Priority, j.Body)
			qm.Lock()
			qm.JobsInProgress[clientAddr] = append(qm.JobsInProgress[clientAddr], j)
			qm.Unlock()
			write(conn, resp, message)
			return
		}

	case "put":
		j := types.NewJob(int(messageJson["pri"].(float64)), messageJson["job"].(map[string]interface{}), messageJson["queue"].(string))
		qm.Lock()
		q, ok := qm.Queues[messageJson["queue"].(string)]
		if !ok {

			qname := messageJson["queue"].(string)
			q = types.NewQueue()
			qm.Queues[qname] = q
		}
		qm.Unlock()

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
		resp := make(map[string]any)
		resp["id"] = id

		found := qm.AbortJob(clientAddr, id)

		if !found {
			resp["status"] = "no-job"
			write(conn, resp, message)
			return
		}

		resp["status"] = "ok"
		write(conn, resp, message)
		return

	case "delete":
		id := strconv.FormatFloat(messageJson["id"].(float64), 'f', -1, 64)
		resp := make(map[string]any)

		// make sure this is a valid job id
		qm.Lock()
		job, ok := qm.Jobs[id]
		qm.Unlock()
		if !ok {
			resp["status"] = "no-job"
			write(conn, resp, message)
			return
		}

		if job.Deleted {
			resp["status"] = "no-job"
			write(conn, resp, message)
			return
		}

		// delete this job if it was found in a queue
		qm.DeleteJob(job)
		resp["id"] = id
		resp["status"] = "ok"
		write(conn, resp, message)
		return
	}
}

func abort(clientAddr string) {
	defer qm.Unlock()
	qm.Lock()
	clientJobs := qm.JobsInProgress[clientAddr]
	for _, j := range clientJobs {
		// Indicate that this job is not being handled and out it back in its queue
		j.Client = ""
		qm.Queues[j.Queue].JobsOrdered = append(qm.Queues[j.Queue].JobsOrdered, j)
		sort.Sort(qm.Queues[j.Queue].JobsOrdered)
	}
	delete(qm.JobsInProgress, clientAddr)
}

func write(conn net.Conn, r map[string]any, message string) {
	responseJson, _ := json.Marshal(r)
	io.WriteString(conn, string(responseJson))
}
