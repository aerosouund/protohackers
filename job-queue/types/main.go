package types

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
)

// max heap

type Queue struct {
	JobsLookup map[string]*Job

	JobsMu      sync.Mutex
	JobsOrdered SortedJobs
	Heap        []int
}

func (q *Queue) PutJob(j *Job) {
	q.JobsLookup[j.ID] = j
	q.JobsMu.Lock()
	q.JobsOrdered = append(q.JobsOrdered, *j)
	q.JobsMu.Unlock()
	sort.Sort(q.JobsOrdered)
}

func (q *Queue) GetJob() *Job {
	if len(q.JobsOrdered) == 0 {
		return nil
	}
	n := 0
	for i := 0; i < q.JobsOrdered.Len(); i++ {
		if j := q.JobsLookup[q.JobsOrdered[n].ID]; j.Client != "" {
			n += 1
		} else {
			return q.JobsLookup[q.JobsOrdered[n].ID]
		}
	}
	return nil // should this order be reversed ?
}

func (q *Queue) DeleteJob(id string) error {
	if len(q.JobsOrdered) == 1 && q.JobsOrdered[0].ID == id {
		q.JobsOrdered = SortedJobs{}
	}

	for i, _ := range q.JobsOrdered {
		if q.JobsOrdered[i].ID == id {
			q.JobsOrdered = append(q.JobsOrdered[:i], q.JobsOrdered[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("Job not found in queue")
}

type SortedJobs []Job

func (sj SortedJobs) Len() int {
	return len(sj)
}

func (sj SortedJobs) Less(i, j int) bool {
	return sj[j].Priority < sj[i].Priority
}

func (sj SortedJobs) Swap(i, j int) {
	sj[i], sj[j] = sj[j], sj[i]
}

type Job struct {
	ID       string
	Priority int
	Body     map[string]interface{}

	ClientMu sync.Mutex
	Client   string

	Queue string
}

func NewJob(pri int, body map[string]interface{}, queue string) *Job {
	id := rand.Intn(100000)
	return &Job{
		ID:       strconv.Itoa(id),
		Priority: pri,
		Body:     body,
		Queue:    queue,
	}
}

type QueueManager struct {
	Queues map[string]*Queue

	JobsMu sync.Mutex

	// it is this way because when we try to delete a job we need a reference to the queue it belongs to
	JobsInProgress map[string]map[string]*Job
}

func NewQueueManager() *QueueManager {
	return &QueueManager{
		Queues:         make(map[string]*Queue),
		JobsInProgress: make(map[string]map[string]*Job), // from client address to job ids to pointer to job
	}
}

func NewQueue() *Queue {
	return &Queue{
		JobsLookup: make(map[string]*Job),
	}
}

func NewResponse(status, queue, id string, pri int, job map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status": status,
		"queue":  queue,
		"job":    job,
		"id":     id,
		"pri":    pri,
	}
}

var ErrQueueNotFound = errors.New("Attempting to get jobs from a queue that doesn't exist")
var ErrRequestNotSupported = errors.New("Request type is invalid")
var ErrInvalidMessage = errors.New("Invalid message format")

func insert(pri int, q Queue) {
	q.Heap = append(q.Heap, pri)
	up(pri, q)
}

func pop(q Queue) int {
	q.Heap[0], q.Heap[len(q.Heap)-1] = q.Heap[len(q.Heap)-1], q.Heap[0]
	retVal := q.Heap[len(q.Heap)-1]
	down(q)
	return retVal
}

func up(pri int, q Queue) {
	i := len(q.Heap) - 1
	for {
		if pri < q.Heap[(i-1)/2] {
			i = q.Heap[(i-1)/2]
			q.Heap[(i-1)/2] = pri
		}
	}
}

func down(q Queue) {
	i := q.Heap[0]
	r := q.Heap[1]
	l := q.Heap[2]

	for {
		var maxIdx int
		maxIdx = l
		if q.Heap[r] > q.Heap[l] {
			maxIdx = r
		}
		if i < q.Heap[maxIdx] {
			q.Heap[i], q.Heap[maxIdx] = q.Heap[maxIdx], q.Heap[i]

			i = maxIdx
			l = 2*i + 1
			r = 2*i + 2
		}
	}
}
