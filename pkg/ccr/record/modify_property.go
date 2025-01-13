package record

import (
	"encoding/json"
	"fmt"

	"github.com/selectdb/ccr_syncer/pkg/xerror"
)

type ModifyTableProperty struct {
	DbId       int64             `json:"dbId"`
	TableId    int64             `json:"tableId"`
	TableName  string            `json:"tableName"`
	Properties map[string]string `json:"properties"`
	Sql        string            `json:"sql"`
}

func NewModifyTablePropertyFromJson(data string) (*ModifyTableProperty, error) {
	var modifyProperty ModifyTableProperty
	err := json.Unmarshal([]byte(data), &modifyProperty)
	if err != nil {
		return nil, xerror.Wrap(err, xerror.Normal, "unmarshal modify table property error")
	}

	if modifyProperty.TableId == 0 {
		return nil, xerror.Errorf(xerror.Normal, "table id not found")
	}

	return &modifyProperty, nil
}

func (m *ModifyTableProperty) String() string {
	return fmt.Sprintf("ModifyTableProperty: DbId: %d, TableId: %d, TableName: %s, Properties: %v, Sql: %s",
		m.DbId, m.TableId, m.TableName, m.Properties, m.Sql)
}
