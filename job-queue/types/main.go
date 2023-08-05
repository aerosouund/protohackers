package types

import (
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"time"
)

var ErrMap = map[string]any{
	"status": "error",
}

type Queue struct {
	LookupMu   sync.Mutex
	JobsLookup map[string]*Job

	JobsMu      sync.Mutex
	JobsOrdered SortedJobs
	Heap        []int
}

func (qm *QueueManager) PutJob(queue *Queue, j *Job) {
	// defer q.JobsMu.Unlock()
	defer qm.Unlock()

	// q.JobsMu.Lock()
	qm.Lock()

	queue.JobsLookup[j.ID] = j
	queue.JobsOrdered = append(queue.JobsOrdered, j)
	sort.Sort(queue.JobsOrdered)

}

func (qm *QueueManager) GetJob(clientAddr string, queues []string) *Job {
	defer qm.Unlock()

	qm.Lock()
	var maxPriJob = &Job{Priority: -1}
	for _, queue := range queues {
		q := qm.Queues[queue]
		if q != nil && len(q.JobsOrdered) > 0 {
			n := 0
			// q.JobsMu.Lock()
			for i := 0; i < q.JobsOrdered.Len(); i++ {
				// j.Client != ""
				if j := q.JobsLookup[q.JobsOrdered[n].ID]; j.Priority < 0 || j.Deleted {
					n += 1
				} else {
					if q.JobsLookup[q.JobsOrdered[n].ID].Priority > maxPriJob.Priority {
						maxPriJob = q.JobsLookup[q.JobsOrdered[n].ID]
						break
					}
				}
			}
		}
	}
	if maxPriJob.Priority == -1 {
		return nil
	}
	maxPriJob.Priority = -maxPriJob.Priority
	return maxPriJob
}

func (qm *QueueManager) DeleteJob(j *Job) {
	qm.Lock()
	j.Deleted = true
	qm.Unlock()
}

type SortedJobs []*Job

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
	sync.Mutex
	ID       string
	Priority int
	Body     map[string]interface{}
	Deleted  bool

	Client string

	Queue string
}

func NewJob(pri int, body map[string]interface{}, queue string) *Job {
	rand.Seed(time.Now().UnixNano())
	id := rand.Intn(1000000000000)
	return &Job{
		ID:       strconv.Itoa(id),
		Priority: pri,
		Body:     body,
		Queue:    queue,
	}
}

type QueueManager struct {
	sync.Mutex
	Queues map[string]*Queue

	JobsMu sync.Mutex

	// it is this way because when we try to delete a job we need a reference to the queue it belongs to
	JobsInProgress map[string][]*Job
}

func NewQueueManager() *QueueManager {
	return &QueueManager{
		Queues:         make(map[string]*Queue),
		JobsInProgress: make(map[string][]*Job), // from client address to job ids to pointer to job
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
