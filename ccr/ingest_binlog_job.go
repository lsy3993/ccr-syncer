package ccr

import (
	"context"
	"sync"

	"github.com/modern-go/gls"
	"github.com/pkg/errors"
	"github.com/selectdb/ccr_syncer/ccr/base"
	"github.com/selectdb/ccr_syncer/ccr/record"
	"github.com/selectdb/ccr_syncer/rpc"
	bestruct "github.com/selectdb/ccr_syncer/rpc/kitex_gen/backendservice"
	tstatus "github.com/selectdb/ccr_syncer/rpc/kitex_gen/status"
	ttypes "github.com/selectdb/ccr_syncer/rpc/kitex_gen/types"
	utils "github.com/selectdb/ccr_syncer/utils"
	log "github.com/sirupsen/logrus"
)

type IngestContext struct {
	context.Context
	txnId        int64
	tableRecords []*record.TableRecord
}

func NewIngestContext(txnId int64, tableRecords []*record.TableRecord) *IngestContext {
	return &IngestContext{
		Context:      context.Background(),
		txnId:        txnId,
		tableRecords: tableRecords,
	}
}

type IngestBinlogJob struct {
	job          *Job // ccr job
	txnId        int64
	tableRecords []*record.TableRecord

	srcBackendMap  map[int64]*base.Backend
	destBackendMap map[int64]*base.Backend

	commitInfos     []*ttypes.TTabletCommitInfo
	commitInfosLock sync.Mutex

	err     error
	errLock sync.RWMutex

	wg sync.WaitGroup
}

func NewIngestBinlogJob(ctx context.Context, job *Job) (*IngestBinlogJob, error) {
	// convert ctx to IngestContext
	ingestCtx, ok := ctx.(*IngestContext)
	if !ok {
		return nil, errors.Errorf("invalid context type: %T", ctx)
	}

	return &IngestBinlogJob{
		job:          job,
		txnId:        ingestCtx.txnId,
		tableRecords: ingestCtx.tableRecords,
		commitInfos:  make([]*ttypes.TTabletCommitInfo, 0),
	}, nil
}

func (j *IngestBinlogJob) GetTabletCommitInfos() []*ttypes.TTabletCommitInfo {
	return j.commitInfos
}

func (j *IngestBinlogJob) setError(err error) {
	j.errLock.Lock()
	defer j.errLock.Unlock()

	j.err = err
}

func (j *IngestBinlogJob) Error() error {
	j.errLock.RLock()
	defer j.errLock.RUnlock()

	return j.err
}

func (j *IngestBinlogJob) appendCommitInfos(commitInfo *ttypes.TTabletCommitInfo) {
	j.commitInfosLock.Lock()
	defer j.commitInfosLock.Unlock()

	j.commitInfos = append(j.commitInfos, commitInfo)
}

func (j *IngestBinlogJob) CommitInfos() []*ttypes.TTabletCommitInfo {
	j.commitInfosLock.Lock()
	defer j.commitInfosLock.Unlock()

	return j.commitInfos
}

func (j *IngestBinlogJob) handleReplica(binlogVersion int64, srcTablet *TabletMeta, destPartitionId int64, destReplicaId int64, destReplica *ReplicaMeta) bool {
	log.Debugf("handle dest replica id: %v", destReplicaId)
	destBackend, ok := j.destBackendMap[destReplica.BackendId]
	if !ok {
		j.setError(errors.Errorf("backend not found, backend id: %d", destReplica.BackendId))
		return false
	}
	destTabletId := destReplica.TabletId

	destRpc, err := rpc.NewBeRpc(destBackend)
	if err != nil {
		j.setError(err)
		return false
	}
	loadId := ttypes.NewTUniqueId()
	loadId.SetHi(-1)
	loadId.SetLo(-1)

	srcReplicas := srcTablet.ReplicaMetas
	iter := srcReplicas.Iter()
	if ok := iter.First(); !ok {
		j.setError(errors.Errorf("src replicas is empty"))
		return false
	}
	srcBackendId := iter.Value().BackendId
	var srcBackend *base.Backend
	srcBackend, ok = j.srcBackendMap[srcBackendId]
	if !ok {
		j.setError(errors.Errorf("backend not found, backend id: %d", srcBackendId))
		return false
	}
	req := &bestruct.TIngestBinlogRequest{
		TxnId:          utils.ThriftValueWrapper(j.txnId),
		RemoteTabletId: utils.ThriftValueWrapper[int64](srcTablet.Id),
		BinlogVersion:  utils.ThriftValueWrapper(binlogVersion),
		RemoteHost:     utils.ThriftValueWrapper(srcBackend.Host),
		RemotePort:     utils.ThriftValueWrapper(srcBackend.GetHttpPortStr()),
		PartitionId:    utils.ThriftValueWrapper[int64](destPartitionId),
		LocalTabletId:  utils.ThriftValueWrapper[int64](destTabletId),
		LoadId:         loadId,
	}
	commitInfo := &ttypes.TTabletCommitInfo{
		TabletId:  destTabletId,
		BackendId: destBackend.Id,
	}

	j.wg.Add(1)
	go func() {
		gls.ResetGls(gls.GoID(), map[interface{}]interface{}{})
		gls.Set("job", j.job.Name)

		defer j.wg.Done()

		resp, err := destRpc.IngestBinlog(req)
		if err != nil {
			j.setError(err)
			return
		}

		log.Infof("ingest resp: %v", resp)
		if !resp.IsSetStatus() {
			err = errors.Errorf("ingest resp status not set")
			j.setError(err)
			return
		} else if resp.Status.StatusCode != tstatus.TStatusCode_OK {
			err = errors.Errorf("ingest resp status code: %v, msg: %v", resp.Status.StatusCode, resp.Status.ErrorMsgs)
			j.setError(err)
			return
		} else {
			j.appendCommitInfos(commitInfo)
		}
	}()

	return true
}

func (j *IngestBinlogJob) handleTablet(binlogVersion int64, srcTablet *TabletMeta, destPartitionId int64, destTablet *TabletMeta) {
	destTablet.ReplicaMetas.Scan(func(destReplicaId int64, destReplica *ReplicaMeta) bool {
		return j.handleReplica(binlogVersion, srcTablet, destPartitionId, destReplicaId, destReplica)
	})
}

func (j *IngestBinlogJob) handleIndex() {
}

func (j *IngestBinlogJob) handlePartition(srcTableId, destTableId int64, partitionRecord record.PartitionRecord) {
	log.Debugf("partitionRecord: %v", partitionRecord)

	job := j.job
	binlogVersion := partitionRecord.Version
	srcPartitionId := partitionRecord.PartitionID
	srcPartitionRange, err := job.srcMeta.GetPartitionRange(srcTableId, srcPartitionId)
	if err != nil {
		j.setError(err)
		return
	}
	var destPartitionId int64
	destPartitionId, err = job.destMeta.GetPartitionIdByRange(destTableId, srcPartitionRange)
	if err != nil {
		j.setError(err)
		return
	}

	var srcTablets []*TabletMeta
	srcTablets, err = job.srcMeta.GetTabletList(srcTableId, srcPartitionId)
	if err != nil {
		j.setError(err)
		return
	}
	var destTablets []*TabletMeta
	destTablets, err = job.destMeta.GetTabletList(destTableId, destPartitionId)
	if err != nil {
		j.setError(err)
		return
	}
	if len(srcTablets) != len(destTablets) {
		j.setError(errors.Errorf("tablet count not match, src: %d, dest: %d", len(srcTablets), len(destTablets)))
		return
	}

	for tabletIndex, destTablet := range destTablets {
		srcTablet := srcTablets[tabletIndex]
		log.Debugf("handle tablet index: %v, src tablet: %v, dest tablet: %v, dest replicas length: %d", tabletIndex, srcTablet, destTablet, destTablet.ReplicaMetas.Len()) // TODO: remove it

		// iterate dest replicas
		j.handleTablet(binlogVersion, srcTablet, destPartitionId, destTablet)
		if j.Error() != nil {
			return
		}
	}
}

func (j *IngestBinlogJob) handleTable(tableRecord *record.TableRecord) {
	log.Debugf("tableRecord: %v", tableRecord)
	job := j.job
	// TODO: check it before ingestBinlog
	var srcTableId int64
	var destTableId int64

	var err error
	switch job.SyncType {
	case TableSync:
		srcTableId = job.Src.TableId
		destTableId = job.Dest.TableId
	case DBSync:
		srcTableId = tableRecord.Id
		destTableId, err = job.getDestTableIdBySrc(tableRecord.Id)
		if err != nil {
			break
		}
	default:
		err = errors.Errorf("invalid sync type: %s", job.SyncType)
	}
	if err != nil {
		j.setError(err)
		return
	}

	// srcPartitionMap, err := job.srcMeta.GetPartitionRangeMap(srcTableId)
	// if err != nil {
	// 	j.setError(err)
	// 	return
	// }
	// destPartitionMap, err := job.destMeta.GetPartitionRangeMap(destTableId)
	// if err != nil {
	// 	j.setError(err)
	// 	return
	// }

	for _, partitionRecord := range tableRecord.PartitionRecords {
		j.handlePartition(srcTableId, destTableId, partitionRecord)
	}
}

func (j *IngestBinlogJob) prepareBackendMap() error {
	job := j.job

	var err error
	j.srcBackendMap, err = job.srcMeta.GetBackendMap()
	if err != nil {
		return err
	}

	j.destBackendMap, err = job.destMeta.GetBackendMap()
	if err != nil {
		return err
	}

	return nil
}

func (j *IngestBinlogJob) run() {
	if err := j.prepareBackendMap(); err != nil {
		j.setError(err)
		return
	}

	for _, tableRecord := range j.tableRecords {
		j.handleTable(tableRecord)
		if j.Error() != nil {
			return
		}
	}
}

func (j *IngestBinlogJob) Run() {
	j.run()
	j.wg.Wait()
}
