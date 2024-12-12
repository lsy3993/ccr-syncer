package storage

import (
	"errors"
	"flag"
)

var (
	ErrJobExists    = errors.New("job exists")
	ErrJobNotExists = errors.New("job not exists")
)

const (
	InvalidCheckTimestamp    int64  = -1
	remoteDBName             string = "ccr"
	defaultMaxOpenConnctions int    = 20
)

var maxOpenConnctions int
var maxAllowedPacket int64

func init() {
	flag.Int64Var(&maxAllowedPacket, "mysql_max_allowed_packet", defaultMaxAllowedPacket,
		"Config the max allowed packet to send to mysql server, the upper limit is 1GB")
	flag.IntVar(&maxOpenConnctions, "max_open_connection", defaultMaxOpenConnctions,
		"Config the max open connections for db user")
}

type DB interface {
	// Add ccr job
	AddJob(jobName string, jobInfo string, hostInfo string) error
	// Update ccr job
	UpdateJob(jobName string, jobInfo string) error
	// Remove ccr job
	RemoveJob(jobName string) error
	// Check Job exist
	IsJobExist(jobName string) (bool, error)
	// Get job_info
	GetJobInfo(jobName string) (string, error)
	// Get job_belong
	GetJobBelong(jobName string) (string, error)

	// Update ccr sync progress
	UpdateProgress(jobName string, progress string) error
	// IsProgressExist
	IsProgressExist(jobName string) (bool, error)
	// Get ccr sync progress
	GetProgress(jobName string) (string, error)

	// AddSyncer
	AddSyncer(hostInfo string) error
	// RefreshSyncer
	RefreshSyncer(hostInfo string, lastStamp int64) (int64, error)
	// GetStampAndJobs
	GetStampAndJobs(hostInfo string) (int64, []string, error)
	// GetOrphanJobs
	GetDeadSyncers(expiredTime int64) ([]string, error)
	// rebalance load
	RebalanceLoadFromDeadSyncers(syncers []string) error

	// GetAllData
	GetAllData() (map[string][]string, error)
}
