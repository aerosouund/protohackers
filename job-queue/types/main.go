package types

import (
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"time"
)

type QueueManager struct {
	sync.Mutex
	Queues map[string]*Queue
	Jobs   map[string]*Job

	// map from client ID to a slice of the jobs they handle
	JobsInProgress map[string][]*Job
}

func (qm *QueueManager) PutJob(queue *Queue, j *Job) {
	defer qm.Unlock()
	qm.Lock()

	// put the job in the lookup map for this queue and in the map of all jobs then sort the queue
	queue.JobsLookup[j.ID] = j
	qm.Jobs[j.ID] = j
	queue.JobsOrdered = append(queue.JobsOrdered, j)
	sort.Sort(queue.JobsOrdered)
}

func (qm *QueueManager) GetJob(clientAddr string, queues []string) *Job {
	defer qm.Unlock()

	qm.Lock()
	var maxPriJob = &Job{Priority: -1}
	var idx int
	for _, queue := range queues {
		q := qm.Queues[queue]
		// the queue exists and has jobs
		if q != nil && len(q.JobsOrdered) > 0 {
			n := 0
			for i := 0; i < q.JobsOrdered.Len(); i++ {
				// the job is not being handled and isn't deleted
				if j := q.JobsLookup[q.JobsOrdered[n].ID]; j.Client != "" || j.Deleted {
					n += 1
				} else {
					// get the first max priority job from that queue that has a priority > max priority job
					if q.JobsLookup[q.JobsOrdered[n].ID].Priority > maxPriJob.Priority {
						maxPriJob = q.JobsLookup[q.JobsOrdered[n].ID]
						idx = n
						break
					}
				}
			}
		}
	}
	// handle no job being found
	if maxPriJob.Priority == -1 {
		return nil
	}
	// remove the job from the queue
	maxPriJob.Client = clientAddr
	q := qm.Queues[maxPriJob.Queue]
	q.JobsOrdered = append(q.JobsOrdered[:idx], q.JobsOrdered[idx+1:]...)
	return maxPriJob
}

func (qm *QueueManager) DeleteJob(j *Job) {
	qm.Lock()
	j.Deleted = true
	qm.Unlock()
}

func (qm *QueueManager) AbortJob(clientAddr string, jobID string) bool {
	defer qm.Unlock()
	qm.Lock()

	var found = false
	for i, job := range qm.JobsInProgress[clientAddr] {
		if job.ID == jobID && !job.Deleted {
			// put the job back in its queue and sort the queue
			found = true
			qm.Queues[job.Queue].JobsOrdered = append(qm.Queues[job.Queue].JobsOrdered, job)
			job.Client = ""
			qm.JobsInProgress[clientAddr][i] = qm.JobsInProgress[clientAddr][0]
			qm.JobsInProgress[clientAddr] = qm.JobsInProgress[clientAddr][1:]

			sort.Sort(qm.Queues[job.Queue].JobsOrdered)

		}
	}
	return found

}

type Job struct {
	ID       string
	Priority int
	Body     map[string]interface{}
	Deleted  bool
	Client   string

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

type Queue struct {
	JobsLookup  map[string]*Job
	JobsOrdered SortedJobs
}

func NewQueueManager() *QueueManager {
	return &QueueManager{
		Queues:         make(map[string]*Queue),
		Jobs:           make(map[string]*Job),
		JobsInProgress: make(map[string][]*Job),
	}
}

func NewQueue() *Queue {
	return &Queue{
		JobsLookup: make(map[string]*Job),
	}
}

// implement the sort interface on a slice of pointer to job
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

func NewResponse(status, queue, id string, pri int, job map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status": status,
		"queue":  queue,
		"job":    job,
		"id":     id,
		"pri":    pri,
	}
}

var ErrMap = map[string]any{
	"status": "error",
}

var ErrQueueNotFound = errors.New("Attempting to get jobs from a queue that doesn't exist")
var ErrRequestNotSupported = errors.New("Request type is invalid")
var ErrInvalidMessage = errors.New("Invalid message format")
