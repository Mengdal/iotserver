package collectapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
)

const (
	collectPointsSheet  = "点位"
	collectExportName   = "CollectPoints"
	collectExcelFileErr = "读取 Excel 失败"
)

// 导出表头与 Excel 示例保持一致，额外增加「使能」列。
var collectPointHeaders = []interface{}{
	"标签ID", "通道", "地址", "数据类型", "读写", "倍率/偏移", "存储", "使能",
}

// CollectConfigExport 导出设备点表为 xlsx；点表为空时导出物模型模板。
func (s *service) CollectConfigExport(ctx context.Context, deviceId string) (*dtos.ExportFile, error) {
	config, err := s.CollectConfigGet(ctx, deviceId)
	if err != nil {
		return nil, err
	}
	file, err := dtos.NewExportFile(collectExportName)
	if err != nil {
		return nil, err
	}
	sheet := file.Excel.GetSheetName(0)
	if sheet == "" {
		sheet = "Sheet1"
	}
	file.Excel.SetSheetName(sheet, collectPointsSheet)
	sheet = collectPointsSheet
	if err := file.Excel.SetSheetRow(sheet, "A1", &collectPointHeaders); err != nil {
		return nil, err
	}
	for index, point := range config.Points {
		row := []interface{}{
			point.TagId,
			config.ChannelName,
			point.Address,
			point.DataType,
			point.AccessMode,
			scaleOffset(point.Multiplier, point.Offset),
			excelBool(point.Storage),
			excelBool(point.Enabled),
		}
		if err := file.Excel.SetSheetRow(sheet, fmt.Sprintf("A%d", index+2), &row); err != nil {
			return nil, err
		}
	}
	headerStyle, err := file.Excel.NewStyle(`{"font":{"bold":true},"alignment":{"horizontal":"center","vertical":"center"}}`)
	if err == nil {
		_ = file.Excel.SetCellStyle(sheet, "A1", "H1", headerStyle)
	}
	widths := map[string]float64{"A": 20, "B": 10, "C": 26, "D": 12, "E": 8, "F": 12, "G": 8, "H": 8}
	for column, width := range widths {
		_ = file.Excel.SetColWidth(sheet, column, column, width)
	}
	return file, nil
}

// CollectConfigImport 解析导入的 xlsx，返回点位列表；由前端确认后再保存。
// 支持示例格式（倍率/偏移 合并列），也兼容拆开的倍率、偏移列。
func (s *service) CollectConfigImport(ctx context.Context, deviceId string, importFile *dtos.ImportFile) ([]dtos.CollectPointDTO, error) {
	iotpId, err := parsePositiveId(deviceId, "device id")
	if err != nil {
		return nil, err
	}
	client, err := s.engineClient()
	if err != nil {
		return nil, err
	}
	binding, _, found, err := s.bindingForIotp(client, iotpId)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备未绑定通道"))
	}
	if ok, err := s.canAccessChannel(ctx, binding.ChannelId); err != nil {
		return nil, err
	} else if !ok {
		return nil, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备未绑定通道"))
	}

	sheetName := importFile.Excel.GetSheetName(0)
	if sheetName == "" {
		return nil, excelErr(fmt.Errorf("Excel 中没有工作表"))
	}
	rows, err := importFile.Excel.GetRows(sheetName)
	if err != nil {
		return nil, excelErr(err)
	}
	if len(rows) == 0 {
		return []dtos.CollectPointDTO{}, nil
	}
	return parseCollectPointRows(rows)
}

// parseCollectPointRows 把 Excel 行解析为点位；首行为表头。
func parseCollectPointRows(rows [][]string) ([]dtos.CollectPointDTO, error) {
	if len(rows) == 0 {
		return []dtos.CollectPointDTO{}, nil
	}
	columns := collectImportColumns(rows[0])
	if columns.tag < 0 || columns.address < 0 || columns.dataType < 0 {
		return nil, errort.NewCommonErr(errort.DefaultReadExcelErrorCode,
			fmt.Errorf("Excel 缺少必需列：标签ID / 地址 / 数据类型"))
	}

	points := make([]dtos.CollectPointDTO, 0, len(rows)-1)
	for index, row := range rows[1:] {
		line := index + 2
		if excelRowEmpty(row) {
			continue
		}
		tagId := strings.TrimSpace(excelCell(row, columns.tag))
		address := strings.TrimSpace(excelCell(row, columns.address))
		dataType := normalizeDataType(excelCell(row, columns.dataType))
		if tagId == "" && address == "" && dataType == "" {
			continue
		}
		if tagId == "" {
			return nil, excelRowErr(line, "标签ID不能为空")
		}
		if address == "" {
			return nil, excelRowErr(line, fmt.Sprintf("（%s）地址不能为空", tagId))
		}
		if dataType == "" {
			return nil, excelRowErr(line, fmt.Sprintf("（%s）数据类型不能为空", tagId))
		}
		accessMode := strings.ToUpper(strings.TrimSpace(excelCell(row, columns.accessMode)))
		if accessMode == "" {
			accessMode = "R"
		}
		switch accessMode {
		case "R", "W", "RW":
		default:
			return nil, excelRowErr(line, fmt.Sprintf("（%s）读写类型 %s 不支持", tagId, accessMode))
		}

		multiplier, offset := 1.0, 0.0
		var err error
		if columns.multiplier >= 0 || columns.offset >= 0 {
			if multiplier, err = excelFloat(excelCell(row, columns.multiplier), 1); err != nil {
				return nil, excelRowErr(line, fmt.Sprintf("（%s）倍率格式不正确", tagId))
			}
			if offset, err = excelFloat(excelCell(row, columns.offset), 0); err != nil {
				return nil, excelRowErr(line, fmt.Sprintf("（%s）偏移格式不正确", tagId))
			}
		} else if columns.scale >= 0 {
			if multiplier, offset, err = parseExcelScaleOffset(excelCell(row, columns.scale)); err != nil {
				return nil, excelRowErr(line, fmt.Sprintf("（%s）倍率/偏移格式不正确", tagId))
			}
		}

		points = append(points, dtos.CollectPointDTO{
			TagId:      tagId,
			Address:    address,
			DataType:   dataType,
			AccessMode: accessMode,
			Multiplier: multiplier,
			Offset:     offset,
			Storage:    excelBoolValue(excelCell(row, columns.storage), true),
			Enabled:    excelBoolValue(excelCell(row, columns.enabled), true),
			Sort:       len(points),
		})
	}
	return points, nil
}

type collectImportColumnIndex struct {
	tag        int
	address    int
	dataType   int
	accessMode int
	multiplier int
	offset     int
	scale      int
	storage    int
	enabled    int
}

func newCollectImportColumns() collectImportColumnIndex {
	return collectImportColumnIndex{
		tag: -1, address: -1, dataType: -1, accessMode: -1,
		multiplier: -1, offset: -1, scale: -1, storage: -1, enabled: -1,
	}
}

func collectImportColumns(header []string) collectImportColumnIndex {
	columns := newCollectImportColumns()
	for index, cell := range header {
		switch normalizeExcelHeader(cell) {
		case "标签id", "标签", "tagid", "点位id", "点位编码", "tagcode":
			columns.tag = index
		case "地址", "address", "点位地址":
			columns.address = index
		case "数据类型", "datatype", "类型":
			columns.dataType = index
		case "读写", "读写模式", "readwrite", "accessmode":
			columns.accessMode = index
		case "倍率", "multiplier", "系数":
			columns.multiplier = index
		case "偏移", "offset", "偏移量":
			columns.offset = index
		case "倍率/偏移", "倍率、偏移", "倍率偏移", "scaleoffset", "缩放偏移":
			columns.scale = index
		case "存储", "storage":
			columns.storage = index
		case "使能", "启用", "enabled", "enable":
			columns.enabled = index
		}
	}
	return columns
}

func normalizeExcelHeader(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "（", "(")
	value = strings.ReplaceAll(value, "）", ")")
	return value
}

func excelCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func excelRowEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func excelBool(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func excelBoolValue(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "是", "true", "1", "yes", "y", "启用", "开启", "使能":
		return true
	case "否", "false", "0", "no", "n", "禁用", "关闭":
		return false
	}
	return fallback
}

func excelFloat(value string, fallback float64) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func parseExcelScaleOffset(value string) (float64, float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 1, 0, nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case '/', ',', '，', ';', '；', '|':
			return true
		}
		return false
	})
	if len(parts) == 0 {
		return 1, 0, nil
	}
	multiplier, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, err
	}
	offset := 0.0
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		offset, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return 0, 0, err
		}
	}
	return multiplier, offset, nil
}

func excelErr(err error) error {
	return errort.NewCommonErr(errort.DefaultReadExcelErrorCode, fmt.Errorf("%s: %w", collectExcelFileErr, err))
}

func excelRowErr(line int, message string) error {
	return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("第 %d 行%s", line, message))
}
