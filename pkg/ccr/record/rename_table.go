package record

import (
	"encoding/json"
	"fmt"

	"github.com/selectdb/ccr_syncer/pkg/xerror"
)

type RenameTable struct {
	DbId            int64  `json:"dbId"`
	TableId         int64  `json:"tableId"`
	IndexId         int64  `json:"indexId"`
	ParititonId     int64  `json:"partitionId"`
	NewTableName    string `json:"newTableName"`
	OldTableName    string `json:"OldTableName"`
	NewRollupName   string `json:"newRollupName"`
	OldRollupName   string `json:"OldRollupName"`
	NewParitionName string `json:"newPartitionName"`
	OldParitionName string `json:"OldParitionName"`
}

func NewRenameTableFromJson(data string) (*RenameTable, error) {
	var renameTable RenameTable
	err := json.Unmarshal([]byte(data), &renameTable)
	if err != nil {
		return nil, xerror.Wrap(err, xerror.Normal, "unmarshal rename table error")
	}

	if renameTable.TableId == 0 {
		return nil, xerror.Errorf(xerror.Normal, "table id not found")
	}

	return &renameTable, nil
}

// Stringer
func (r *RenameTable) String() string {
	return fmt.Sprintf("RenameTable: DbId: %d, TableId: %d, ParititonId: %d, IndexId: %d, NewTableName: %s, OldTableName: %s, NewRollupName: %v, OldRollupName: %v, NewParitionName: %s, OldParitionName: %s", r.DbId, r.TableId, r.ParititonId, r.IndexId, r.NewTableName, r.OldTableName, r.NewRollupName, r.OldRollupName, r.NewParitionName, r.OldParitionName)
}
