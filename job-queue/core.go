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
	var messageInterface interface{}

	json.Unmarshal([]byte(message), &messageInterface)
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
	}
	return nil
}

func handleMessage(clientExitChan chan struct{}, clientAddr string, messageBytes []byte, respCh chan map[string]any, conn net.Conn) {
	message := string(messageBytes)
	fmt.Println(message)
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
			// return resp, nil
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

		// return resp, nil

	case "put":
		j := types.NewJob(int(messageJson["pri"].(float64)), messageJson["job"].(map[string]interface{}), messageJson["queue"].(string))
		q, ok := qm.Queues[messageJson["queue"].(string)]
		if !ok {
			qname := messageJson["queue"].(string)
			q = types.NewQueue()
			qm.Queues[qname] = q
		}

		q.PutJob(j)
		resp := make(map[string]any)
		resp["id"] = j.ID
		resp["status"] = "ok"
		write(conn, resp)

		// return resp, nil

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

			// return resp, nil
		}
		j.Client = ""
		resp["status"] = "ok"
		write(conn, resp)

		// return resp, nil

	case "delete":
		id := strconv.FormatFloat(messageJson["id"].(float64), 'f', -1, 64) // converting the float to a string index

		resp := make(map[string]any)
		j, ok := qm.JobsInProgress[clientAddr][id] // check if this client is handling this job id
		if !ok {
			resp["status"] = "no-job"
			write(conn, resp)

			// return resp, nil
		}

		delete(qm.JobsInProgress[clientAddr], id)
		q := qm.Queues[j.Queue]
		q.DeleteJob(j.ID)

		resp["id"] = id
		resp["status"] = "ok"
		write(conn, resp)

		// return resp, nil
	}
	// return nil, types.ErrRequestNotSupported
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
			fmt.Println("canceling goroutines")
			cancel()
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
		// case <-clientExitChan:
		// 	fmt.Println("canceling goroutines")
		// 	return
		case <-ctx.Done():
			fmt.Println("goroutine exiting")
			return
		default:
			q, ok := qm.Queues[queueName]
			if ok {
				j := q.GetJob()
				if j != nil {
					jobCh <- j
					return
				}
			}
		}
	}
}

func aborter(clientAddr string, clientExitChan chan struct{}) {
	<-clientExitChan
	clientJobs := qm.JobsInProgress[clientAddr]
	for _, v := range clientJobs {
		v.ClientMu.Lock()
		v.Client = ""
		v.ClientMu.Unlock()
	}
	delete(qm.JobsInProgress, clientAddr)
}

// conn net.Conn, respCh chan map[string]any, clientExitChan chan struct{}
func write(conn net.Conn, r map[string]any) {
	responseJson, _ := json.Marshal(r)
	fmt.Fprintf(conn, string(responseJson)+"\n")
	// for {
	// 	select {
	// 	case <-clientExitChan:
	// 		fmt.Println("writer exiting")
	// 		return
	// 	case r := <-respCh:
	// 		time.Sleep(time.Millisecond * 5)
	// 		responseJson, _ := json.Marshal(r)
	// 		fmt.Fprintf(conn, string(responseJson)+"\n")
	// 	}
	// }
}
