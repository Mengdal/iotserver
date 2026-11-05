package collectapp

import (
	"context"
	"fmt"
	"sort"
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

// CollectConfigExport 导出设备点表为 xlsx；点表为空时导出物模型模板。
func (s *service) CollectConfigExport(ctx context.Context, deviceId string) (*dtos.ExportFile, error) {
	config, err := s.CollectConfigGet(ctx, deviceId)
	if err != nil {
		return nil, err
	}
	deviceName := config.DeviceName
	if strings.TrimSpace(deviceName) == "" {
		deviceName = fmt.Sprintf("#%d", config.DeviceId)
	}
	rows := make([][]interface{}, 0, len(config.Points))
	for _, point := range config.Points {
		rows = append(rows, []interface{}{
			deviceName,
			point.TagId,
			point.Address,
			point.DataType,
			point.AccessMode,
			scaleOffset(point.Multiplier, point.Offset),
			excelBool(point.Storage),
			strconv.FormatInt(config.DeviceId, 10),
		})
	}
	return newCollectWorkbook(collectExportName, rows)
}

// newCollectWorkbook 生成统一点表工作簿（表头、样式、列宽、隐藏的设备ID列）。
func newCollectWorkbook(fileName string, rows [][]interface{}) (*dtos.ExportFile, error) {
	file, err := dtos.NewExportFile(fileName)
	if err != nil {
		return nil, err
	}
	sheet := file.Excel.GetSheetName(0)
	if sheet == "" {
		sheet = "Sheet1"
	}
	file.Excel.SetSheetName(sheet, collectPointsSheet)
	sheet = collectPointsSheet
	if err := file.Excel.SetSheetRow(sheet, "A1", &collectAllHeaders); err != nil {
		return nil, err
	}
	for index, row := range rows {
		data := row
		if err := file.Excel.SetSheetRow(sheet, fmt.Sprintf("A%d", index+2), &data); err != nil {
			return nil, err
		}
	}
	headerStyle, err := file.Excel.NewStyle(`{"font":{"bold":true},"alignment":{"horizontal":"center","vertical":"center"}}`)
	if err == nil {
		_ = file.Excel.SetCellStyle(sheet, "A1", "H1", headerStyle)
	}
	widths := map[string]float64{"A": 18, "B": 20, "C": 26, "D": 10, "E": 8, "F": 12, "G": 8, "H": 14}
	for column, width := range widths {
		_ = file.Excel.SetColWidth(sheet, column, column, width)
	}
	_ = file.Excel.SetColVisible(sheet, "H", false)
	return file, nil
}

// parseCollectPointRow 解析并校验一行点位；返回错误文案便于批量导入按行汇总。
func parseCollectPointRow(row []string, columns collectImportColumnIndex) (dtos.CollectPointDTO, string) {
	tagId := strings.TrimSpace(excelCell(row, columns.tag))
	address := strings.TrimSpace(excelCell(row, columns.address))
	dataType := normalizeDataType(excelCell(row, columns.dataType))
	if tagId == "" {
		return dtos.CollectPointDTO{}, "标签ID不能为空"
	}
	if address == "" {
		return dtos.CollectPointDTO{}, fmt.Sprintf("（%s）地址不能为空", tagId)
	}
	if dataType == "" {
		return dtos.CollectPointDTO{}, fmt.Sprintf("（%s）数据类型不能为空", tagId)
	}
	accessMode := strings.ToUpper(strings.TrimSpace(excelCell(row, columns.accessMode)))
	if accessMode == "" {
		accessMode = "R"
	}
	switch accessMode {
	case "R", "W", "RW":
	default:
		return dtos.CollectPointDTO{}, fmt.Sprintf("（%s）读写类型 %s 不支持", tagId, accessMode)
	}

	multiplier, offset := 1.0, 0.0
	var err error
	if columns.multiplier >= 0 || columns.offset >= 0 {
		if multiplier, err = excelFloat(excelCell(row, columns.multiplier), 1); err != nil {
			return dtos.CollectPointDTO{}, fmt.Sprintf("（%s）倍率格式不正确", tagId)
		}
		if offset, err = excelFloat(excelCell(row, columns.offset), 0); err != nil {
			return dtos.CollectPointDTO{}, fmt.Sprintf("（%s）偏移格式不正确", tagId)
		}
	} else if columns.scale >= 0 {
		if multiplier, offset, err = parseExcelScaleOffset(excelCell(row, columns.scale)); err != nil {
			return dtos.CollectPointDTO{}, fmt.Sprintf("（%s）倍率/偏移格式不正确", tagId)
		}
	}

	return dtos.CollectPointDTO{
		TagId:      tagId,
		Address:    address,
		DataType:   dataType,
		AccessMode: accessMode,
		Multiplier: multiplier,
		Offset:     offset,
		Storage:    excelBoolValue(excelCell(row, columns.storage), true),
		// 使能是设备级状态，点位不再单独保存使能
		Enabled: true,
	}, ""
}

type collectImportColumnIndex struct {
	deviceId   int
	deviceName int
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
		deviceId: -1, deviceName: -1,
		tag: -1, address: -1, dataType: -1, accessMode: -1,
		multiplier: -1, offset: -1, scale: -1, storage: -1, enabled: -1,
	}
}

func collectImportColumns(header []string) collectImportColumnIndex {
	columns := newCollectImportColumns()
	for index, cell := range header {
		switch normalizeExcelHeader(cell) {
		case "设备id", "设备编号", "iotpid", "deviceid":
			columns.deviceId = index
		case "设备名称", "设备", "devicename", "devname":
			columns.deviceName = index
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

const collectAllExportName = "CollectPointsAll"

var collectAllHeaders = []interface{}{
	"设备名称", "标签ID", "地址", "数据类型", "读写", "倍率/偏移", "存储", "设备ID",
}

type collectExportRow struct {
	channelName string
	deviceName  string
	iotpId      int64
	point       dtos.CollectPointDTO
}

// CollectConfigExportAll 导出当前用户可访问的全部设备点位；channelId 非空时只导出该通道。
func (s *service) CollectConfigExportAll(ctx context.Context, channelId string) (*dtos.ExportFile, error) {
	channels, err := s.CollectChannels(ctx)
	if err != nil {
		return nil, err
	}
	channelNames := make(map[int64]string, len(channels))
	for _, channel := range channels {
		channelNames[channel.Id] = channel.Name
	}
	filter := int64(0)
	if strings.TrimSpace(channelId) != "" {
		filter, err = parsePositiveId(channelId, "channel id")
		if err != nil {
			return nil, err
		}
		if _, ok := channelNames[filter]; !ok {
			return nil, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("通道不存在或无权访问"))
		}
	}
	devices, err := s.CollectDevices(ctx)
	if err != nil {
		return nil, err
	}
	db := s.dbClient.WithCtx(ctx)
	rows := make([]collectExportRow, 0, len(devices)*4)
	for _, device := range devices {
		if filter > 0 && device.ChannelId != filter {
			continue
		}
		points, err := db.CollectPointConfigsByDevice(strconv.FormatInt(device.IotpId, 10))
		if err != nil {
			return nil, err
		}
		if len(points) == 0 {
			continue
		}
		channelName := channelNames[device.ChannelId]
		if channelName == "" {
			channelName = fmt.Sprintf("#%d", device.ChannelId)
		}
		for _, point := range points {
			rows = append(rows, collectExportRow{
				channelName: channelName,
				deviceName:  device.DeviceName,
				iotpId:      device.IotpId,
				point: dtos.CollectPointDTO{
					TagId:      point.TagId,
					Address:    point.Address,
					DataType:   normalizeDataType(point.DataType),
					AccessMode: firstNonEmpty(point.AccessMode, "R"),
					Multiplier: point.Multiplier,
					Offset:     point.Offset,
					Storage:    point.Storage,
					Enabled:    point.Enabled,
					Sort:       point.Sort,
				},
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].channelName != rows[j].channelName {
			return rows[i].channelName < rows[j].channelName
		}
		if rows[i].deviceName != rows[j].deviceName {
			return rows[i].deviceName < rows[j].deviceName
		}
		if rows[i].point.Sort != rows[j].point.Sort {
			return rows[i].point.Sort < rows[j].point.Sort
		}
		return rows[i].point.TagId < rows[j].point.TagId
	})

	data := make([][]interface{}, 0, len(rows))
	for _, row := range rows {
		data = append(data, []interface{}{
			row.deviceName,
			row.point.TagId,
			row.point.Address,
			row.point.DataType,
			row.point.AccessMode,
			scaleOffset(row.point.Multiplier, row.point.Offset),
			excelBool(row.point.Storage),
			strconv.FormatInt(row.iotpId, 10),
		})
	}
	return newCollectWorkbook(collectAllExportName, data)
}

type bulkImportRow struct {
	line       int
	deviceId   int64
	deviceName string
	point      dtos.CollectPointDTO
}

type collectImportGroup struct {
	device      dtos.CollectDeviceDTO
	channelName string
	points      []dtos.CollectPointDTO
}

// CollectConfigImportAllPreview 解析批量点表并返回设备分组与行错误，不写库、不下发引擎。
func (s *service) CollectConfigImportAllPreview(ctx context.Context, file *dtos.ImportFile) (dtos.CollectImportPreview, error) {
	preview := dtos.CollectImportPreview{
		Devices: []dtos.CollectImportDevicePreview{},
		Errors:  []dtos.CollectImportRowError{},
	}
	rows, parseErrs, err := s.readBulkImportFile(file)
	if err != nil {
		return preview, err
	}
	groups, resolveErrs, err := s.resolveBulkImport(ctx, rows)
	if err != nil {
		return preview, err
	}
	for _, group := range groups {
		preview.Devices = append(preview.Devices, dtos.CollectImportDevicePreview{
			DeviceId:    group.device.IotpId,
			DeviceName:  group.device.DeviceName,
			ChannelId:   group.device.ChannelId,
			ChannelName: group.channelName,
			Points:      len(group.points),
		})
	}
	preview.Errors = append(parseErrs, resolveErrs...)
	sort.SliceStable(preview.Errors, func(i, j int) bool {
		return preview.Errors[i].Row < preview.Errors[j].Row
	})
	return preview, nil
}

// CollectConfigImportAll 应用批量点表：文件中出现的设备整表覆盖，其他设备不变。
func (s *service) CollectConfigImportAll(ctx context.Context, file *dtos.ImportFile) (dtos.CollectImportResult, error) {
	result := dtos.CollectImportResult{Failed: []dtos.CollectImportDeviceError{}}
	rows, parseErrs, err := s.readBulkImportFile(file)
	if err != nil {
		return result, err
	}
	groups, resolveErrs, err := s.resolveBulkImport(ctx, rows)
	if err != nil {
		return result, err
	}
	if errs := append(parseErrs, resolveErrs...); len(errs) > 0 {
		return result, bulkImportValidationError(errs)
	}
	db := s.dbClient.WithCtx(ctx)
	for _, group := range groups {
		enabled := true
		if cfg, found, cfgErr := db.CollectDeviceConfigByDevice(strconv.FormatInt(group.device.IotpId, 10)); cfgErr != nil {
			result.Failed = append(result.Failed, dtos.CollectImportDeviceError{
				DeviceId: group.device.IotpId, DeviceName: group.device.DeviceName, Message: cfgErr.Error(),
			})
			continue
		} else if found {
			enabled = cfg.Enabled
		}
		saveErr := s.CollectConfigSave(ctx, dtos.CollectConfigSaveRequest{
			DeviceId: group.device.IotpId,
			Enabled:  enabled,
			Points:   group.points,
		})
		if saveErr != nil {
			result.Failed = append(result.Failed, dtos.CollectImportDeviceError{
				DeviceId: group.device.IotpId, DeviceName: group.device.DeviceName, Message: saveErr.Error(),
			})
			continue
		}
		result.Devices++
		result.Points += len(group.points)
	}
	return result, nil
}

func (s *service) readBulkImportFile(file *dtos.ImportFile) ([]bulkImportRow, []dtos.CollectImportRowError, error) {
	sheetName := file.Excel.GetSheetName(0)
	if sheetName == "" {
		return nil, nil, excelErr(fmt.Errorf("Excel 中没有工作表"))
	}
	rows, err := file.Excel.GetRows(sheetName)
	if err != nil {
		return nil, nil, excelErr(err)
	}
	return parseCollectPointRowsBulk(rows)
}

func parseCollectPointRowsBulk(rows [][]string) ([]bulkImportRow, []dtos.CollectImportRowError, error) {
	if len(rows) == 0 {
		return []bulkImportRow{}, []dtos.CollectImportRowError{}, nil
	}
	columns := collectImportColumns(rows[0])
	if columns.tag < 0 || columns.address < 0 || columns.dataType < 0 {
		return nil, nil, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("Excel 缺少必需列：标签ID / 地址 / 数据类型"))
	}
	if columns.deviceId < 0 && columns.deviceName < 0 {
		return nil, nil, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("Excel 缺少设备列：设备ID 或 设备名称"))
	}
	result := make([]bulkImportRow, 0, len(rows)-1)
	errs := make([]dtos.CollectImportRowError, 0)
	addErr := func(line int, message string) {
		errs = append(errs, dtos.CollectImportRowError{Row: line, Message: message})
	}
	for index, row := range rows[1:] {
		line := index + 2
		if excelRowEmpty(row) {
			continue
		}
		deviceIdText := strings.TrimSpace(excelCell(row, columns.deviceId))
		deviceName := strings.TrimSpace(excelCell(row, columns.deviceName))
		tagText := strings.TrimSpace(excelCell(row, columns.tag))
		addressText := strings.TrimSpace(excelCell(row, columns.address))
		dataTypeText := strings.TrimSpace(excelCell(row, columns.dataType))
		if deviceIdText == "" && deviceName == "" && tagText == "" && addressText == "" && dataTypeText == "" {
			continue
		}
		if deviceIdText == "" && deviceName == "" {
			addErr(line, "设备ID或设备名称不能为空")
			continue
		}
		deviceId := int64(0)
		if deviceIdText != "" {
			parsed, err := strconv.ParseInt(deviceIdText, 10, 64)
			if err != nil || parsed <= 0 {
				addErr(line, "设备ID不合法")
				continue
			}
			deviceId = parsed
		}
		point, message := parseCollectPointRow(row, columns)
		if message != "" {
			addErr(line, message)
			continue
		}
		result = append(result, bulkImportRow{
			line:       line,
			deviceId:   deviceId,
			deviceName: deviceName,
			point:      point,
		})
	}
	return result, errs, nil
}

func normalizeImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *service) resolveBulkImport(ctx context.Context, rows []bulkImportRow) ([]collectImportGroup, []dtos.CollectImportRowError, error) {
	groups := []collectImportGroup{}
	if len(rows) == 0 {
		return groups, nil, nil
	}
	channels, err := s.CollectChannels(ctx)
	if err != nil {
		return nil, nil, err
	}
	devices, err := s.CollectDevices(ctx)
	if err != nil {
		return nil, nil, err
	}
	channelNames := make(map[int64]string, len(channels))
	for _, channel := range channels {
		channelNames[channel.Id] = channel.Name
	}
	byId := make(map[int64][]dtos.CollectDeviceDTO, len(devices))
	byName := make(map[string][]dtos.CollectDeviceDTO, len(devices))
	for _, device := range devices {
		byId[device.IotpId] = append(byId[device.IotpId], device)
		nameKey := normalizeImportKey(device.DeviceName)
		byName[nameKey] = append(byName[nameKey], device)
	}
	groupIndex := make(map[int64]int, len(devices))
	tagsByDevice := make(map[int64]map[string]struct{})
	errs := make([]dtos.CollectImportRowError, 0)
	addErr := func(line int, message string) {
		errs = append(errs, dtos.CollectImportRowError{Row: line, Message: message})
	}
	for _, row := range rows {
		var device dtos.CollectDeviceDTO
		found := false
		switch {
		case row.deviceId > 0:
			matches := byId[row.deviceId]
			if len(matches) == 0 {
				addErr(row.line, "设备不存在或无权访问")
				continue
			}
			// 导入以设备ID为准；设备被移动过也能正常导入到当前绑定通道。
			device = matches[0]
			found = true
		default:
			matches := byName[normalizeImportKey(row.deviceName)]
			if len(matches) == 0 {
				addErr(row.line, "设备不存在或无权访问")
				continue
			}
			if len(matches) > 1 {
				addErr(row.line, "设备名称重复，请填写设备ID")
				continue
			}
			device = matches[0]
			found = true
		}
		if !found {
			addErr(row.line, "设备不存在或无权访问")
			continue
		}
		idx, ok := groupIndex[device.IotpId]
		if !ok {
			idx = len(groups)
			groupIndex[device.IotpId] = idx
			groups = append(groups, collectImportGroup{
				device:      device,
				channelName: channelNames[device.ChannelId],
			})
			tagsByDevice[device.IotpId] = make(map[string]struct{})
		}
		if _, exists := tagsByDevice[device.IotpId][row.point.TagId]; exists {
			addErr(row.line, fmt.Sprintf("（%s）标签ID重复", row.point.TagId))
			continue
		}
		tagsByDevice[device.IotpId][row.point.TagId] = struct{}{}
		row.point.Sort = len(groups[idx].points)
		groups[idx].points = append(groups[idx].points, row.point)
	}
	return groups, errs, nil
}

func bulkImportValidationError(errs []dtos.CollectImportRowError) error {
	if len(errs) == 0 {
		return nil
	}
	limit := len(errs)
	if limit > 3 {
		limit = 3
	}
	parts := make([]string, 0, limit+1)
	for _, item := range errs[:limit] {
		parts = append(parts, fmt.Sprintf("第 %d 行：%s", item.Row, item.Message))
	}
	message := strings.Join(parts, "；")
	if len(errs) > limit {
		message += fmt.Sprintf("；另有 %d 条错误", len(errs)-limit)
	}
	return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("导入校验失败：%s", message))
}
