package services

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/models/constants"
	"strings"
)

func ParseThingModelToEntities(
	categoryId int64,
	o orm.Ormer,
	properties *[]*models.Properties,
	events *[]*models.Events,
	actions *[]*models.Actions,
) (string, error) {
	// 查询品类信息
	var category models.Category
	if err := o.QueryTable(new(models.Category)).Filter("id", categoryId).One(&category); err != nil {
		return "", fmt.Errorf("物模型不存在")
	}
	// 查询物模型信息
	var thingModel models.ThingModel
	if err := o.QueryTable(new(models.ThingModel)).Filter("category_key", category.CategoryKey).One(&thingModel); err != nil {
		return "", fmt.Errorf("物模型不存在")
	}
	// 解析ThingModelJSON，提取properties、events、actions
	if thingModel.ThingModelJson != "" {
		var thingModelData map[string]interface{}
		if err := json.Unmarshal([]byte(thingModel.ThingModelJson), &thingModelData); err == nil {
			// 解析properties
			if props, exists := thingModelData["properties"]; exists {
				if propsArray, ok := props.([]interface{}); ok {
					for _, prop := range propsArray {
						if propMap, ok := prop.(map[string]interface{}); ok {
							property := &models.Properties{
								Name:        GetStringValue(propMap, "name"),
								Code:        GetStringValue(propMap, "identifier"),
								Description: GetStringValue(propMap, "description"),
								Require:     GetBoolValue(propMap, "required"),
								AccessMode:  GetAccessMode(GetStringValue(propMap, "rwFlag")),
								Tag:         string(constants.System),
							}

							// 处理TypeSpec
							if dataSpecs, exists := propMap["dataSpecs"]; exists {
								dataType := propMap["dataType"].(string)
								typeSpec := map[string]interface{}{
									"type":  TransformDataTypeToLower(dataType),
									"specs": BuildTypeSpecs(dataType, dataSpecs),
								}
								marshal, _ := json.Marshal(typeSpec)
								property.TypeSpec = string(marshal)
							}
							if dataSpecsList, exists := propMap["dataSpecsList"]; exists {
								dataType := propMap["dataType"].(string)
								typeSpec := map[string]interface{}{
									"type":  TransformDataTypeToLower(dataType),
									"specs": BuildTypeSpecs(dataType, dataSpecsList),
								}
								marshal, _ := json.Marshal(typeSpec)
								property.TypeSpec = string(marshal)
							}

							*properties = append(*properties, property)
						}
					}
				}
			}

			// 解析events
			if evts, exists := thingModelData["events"]; exists {
				if evtsArray, ok := evts.([]interface{}); ok {
					for _, evt := range evtsArray {
						if evtMap, ok := evt.(map[string]interface{}); ok {
							event := &models.Events{
								Name:        GetStringValue(evtMap, "name"),
								Code:        GetStringValue(evtMap, "identifier"),
								Description: GetStringValue(evtMap, "description"),
								Require:     GetBoolValue(evtMap, "required"),
								EventType:   GetEventType(GetStringValue(evtMap, "eventType")),
								Tag:         string(constants.System),
							}
							*events = append(*events, event)
						}
					}
				}
			}

			// 解析services (actions)
			if svcs, exists := thingModelData["services"]; exists {
				if svcsArray, ok := svcs.([]interface{}); ok {
					for _, svc := range svcsArray {
						if svcMap, ok := svc.(map[string]interface{}); ok {
							action := &models.Actions{
								Name:        GetStringValue(svcMap, "serviceName"),
								Code:        GetStringValue(svcMap, "identifier"),
								Description: GetStringValue(svcMap, "description"),
								Require:     GetBoolValue(svcMap, "required"),
								CallType:    GetStringValue(svcMap, "callType"),
								Tag:         string(constants.System),
							}

							//处理actions的inputParams
							if inputParams, exists := svcMap["inputParams"]; exists {
								if inputArray, ok := inputParams.([]interface{}); ok {
									var inputParamsList []models.InputOutput
									for _, input := range inputArray {
										if inputMap, ok := input.(map[string]interface{}); ok {
											inputParam := models.InputOutput{
												Code: GetStringValue(inputMap, "identifier"),
												Name: GetStringValue(inputMap, "name"),
											}

											// 处理TypeSpec - 使用utils.BuildTypeSpecs
											if dataType, exists := inputMap["dataType"]; exists {
												dataTypeStr := dataType.(string)
												dataSpecs, hasDataSpecs := inputMap["dataSpecs"]
												dataSpecsList, hasDataSpecsList := inputMap["dataSpecsList"]

												// 优先使用dataSpecsList，如果没有则使用dataSpecs
												var specsData interface{}
												if hasDataSpecsList && dataSpecsList != nil {
													specsData = dataSpecsList
												} else if hasDataSpecs && dataSpecs != nil {
													specsData = dataSpecs
												}

												if specsData != nil {
													completeTypeSpec := BuildTypeSpecs(dataTypeStr, specsData)
													inputParam.TypeSpec = completeTypeSpec
												}
											}

											inputParamsList = append(inputParamsList, inputParam)
										}
									}

									// 将inputParams转换为JSON字符串
									if len(inputParamsList) > 0 {
										inputParamsJSON, _ := json.Marshal(inputParamsList)
										action.InputParams = string(inputParamsJSON)
									}
								}
							}

							// 处理actions的outputParams
							if outputParams, exists := svcMap["outputParams"]; exists {
								if outputArray, ok := outputParams.([]interface{}); ok {
									var outputParamsList []models.InputOutput
									for _, output := range outputArray {
										if outputMap, ok := output.(map[string]interface{}); ok {
											outputParam := models.InputOutput{
												Code: GetStringValue(outputMap, "identifier"),
												Name: GetStringValue(outputMap, "name"),
											}

											// 处理TypeSpec - 使用utils.BuildTypeSpecs
											if dataType, exists := outputMap["dataType"]; exists {
												dataTypeStr := dataType.(string)
												dataSpecs, hasDataSpecs := outputMap["dataSpecs"]
												dataSpecsList, hasDataSpecsList := outputMap["dataSpecsList"]

												// 优先使用dataSpecsList，如果没有则使用dataSpecs
												var specsData interface{}
												if hasDataSpecsList && dataSpecsList != nil {
													specsData = dataSpecsList
												} else if hasDataSpecs && dataSpecs != nil {
													specsData = dataSpecs
												}

												if specsData != nil {
													completeTypeSpec := BuildTypeSpecs(dataTypeStr, specsData)
													outputParam.TypeSpec = completeTypeSpec
												}
											}

											outputParamsList = append(outputParamsList, outputParam)
										}
									}

									// 将outputParams转换为JSON字符串
									if len(outputParamsList) > 0 {
										outputParamsJSON, _ := json.Marshal(outputParamsList)
										action.OutputParams = string(outputParamsJSON)
									}
								}
							}

							*actions = append(*actions, action)
						}
					}
				}
			}
		}
	}
	return category.CategoryKey, nil
}

// 辅助函数
func GetStringValue(data map[string]interface{}, key string) string {
	if val, exists := data[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func GetBoolValue(data map[string]interface{}, key string) bool {
	if val, exists := data[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func GetAccessMode(rwFlag string) string {
	switch rwFlag {
	case "READ_ONLY":
		return "R"
	case "READ_WRITE":
		return "RW"
	case "WRITE_ONLY":
		return "W"
	default:
		return "RW"
	}
}
func GetEventType(eventType string) string {
	switch eventType {
	case "ALERT_EVENT_TYPE":
		return string(constants.EventTypeAlert)
	case "INFO_EVENT_TYPE":
		return string(constants.EventTypeInfo)
	case "ERROR_EVENT_TYPE":
		return string(constants.EventTypeError)
	default:
		return string(constants.EventTypeInfo)
	}
}

// TransformDataTypeToLower 将数据类型转换为小写
func TransformDataTypeToLower(dataType string) string {
	switch dataType {
	case "INT":
		return "int"
	case "FLOAT", "DOUBLE":
		return "float"
	case "TEXT":
		return "text"
	case "BOOL":
		return "bool"
	case "ENUM":
		return "enum"
	case "ARRAY":
		return "array"
	case "STRUCT":
		return "struct"
	case "DATE":
		return "date"
	default:
		return dataType
	}
}

// BuildTypeSpecs 根据数据类型和数据规格构建specs字符串
func BuildTypeSpecs(dataType string, dataSpecs interface{}) string {
	switch dataType {
	case "INT", "FLOAT", "DOUBLE", "TEXT":
		// 处理数值类型的specs
		if specsMap, ok := dataSpecs.(map[string]interface{}); ok {
			specs := map[string]string{}
			if Max, exists := specsMap["max"]; exists {
				specs["max"] = InterfaceToString(Max)
			}
			if Min, exists := specsMap["min"]; exists {
				specs["min"] = InterfaceToString(Min)
			}
			if step, exists := specsMap["step"]; exists {
				specs["step"] = InterfaceToString(step)
			}
			if unit, exists := specsMap["unit"]; exists {
				specs["unit"] = InterfaceToString(unit)
			}
			if unitName, exists := specsMap["unitName"]; exists {
				specs["unitName"] = InterfaceToString(unitName)
			}
			if length, exists := specsMap["length"]; exists {
				specs["length"] = InterfaceToString(length)
			}
			specsJSON, _ := json.Marshal(specs)
			return string(specsJSON)
		}
	case "BOOL", "ENUM":
		// 处理枚举类的数据类型
		if specsList, ok := dataSpecs.([]interface{}); ok {
			specs := map[string]string{}
			for _, item := range specsList {
				if itemMap, ok := item.(map[string]interface{}); ok {
					valStr := InterfaceToString(itemMap["value"])
					nameStr := InterfaceToString(itemMap["name"])
					specs[valStr] = nameStr
				}
			}
			specsJSON, _ := json.Marshal(specs)
			return string(specsJSON)
		}
	case "ARRAY":
		// 处理数组类型的specs
		if specsMap, ok := dataSpecs.(map[string]interface{}); ok {
			specs := map[string]interface{}{}

			// 数组大小
			if size, exists := specsMap["size"]; exists {
				specs["size"] = InterfaceToString(size)
			}

			// 数组元素类型
			if childDataType, exists := specsMap["childDataType"]; exists {
				specs["item"] = map[string]string{
					"type": strings.ToLower(InterfaceToString(childDataType)),
				}
			}

			specsJSON, _ := json.Marshal(specs)
			return string(specsJSON)
		}

	case "STRUCT":
		// 处理结构体类型的specs
		if specsList, ok := dataSpecs.([]interface{}); ok {
			var structSpecs []map[string]interface{}

			for _, item := range specsList {
				if itemMap, ok := item.(map[string]interface{}); ok {
					structItem := map[string]interface{}{
						"code": InterfaceToString(itemMap["identifier"]),
						"name": InterfaceToString(itemMap["childName"]),
						"data_type": map[string]interface{}{
							"type": TransformDataTypeToLower(InterfaceToString(itemMap["childDataType"])),
						},
					}
					structSpecs = append(structSpecs, structItem)
				}
			}

			specsJSON, _ := json.Marshal(structSpecs)
			return string(specsJSON)
		}

	case "DATE":
		// 处理日期类型的specs
		specs := map[string]interface{}{}
		specsJSON, _ := json.Marshal(specs)
		return string(specsJSON)

	}
	return ""
}

// InterfaceToString 将任意类型转换为字符串
func InterfaceToString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float64:
		// 检查是否为整数值，如果是则格式化为整数字符串
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		// 否则使用g格式，自动选择最合适的表示方式
		return fmt.Sprintf("%g", v)
	case float32:
		// 检查是否为整数值，如果是则格式化为整数字符串
		if v == float32(int32(v)) {
			return fmt.Sprintf("%d", int32(v))
		}
		// 否则使用g格式，自动选择最合适的表示方式
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
