package controller

func generateAPIConfig(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		result[k] = v
	}

	apiInbound := map[string]interface{}{
		"tag":      "REMNAWAVE_API_INBOUND",
		"port":     APIPort,
		"listen":   "127.0.0.1",
		"protocol": "dokodemo-door",
		"settings": map[string]interface{}{
			"address": "127.0.0.1",
		},
	}

	inbounds, ok := result["inbounds"].([]interface{})
	if !ok {
		inbounds = []interface{}{}
	}

	hasAPIInbound := false
	for _, inbound := range inbounds {
		if ib, ok := inbound.(map[string]interface{}); ok {
			if tag, ok := ib["tag"].(string); ok && tag == "REMNAWAVE_API_INBOUND" {
				hasAPIInbound = true
				break
			}
		}
	}

	if !hasAPIInbound {
		inbounds = append(inbounds, apiInbound)
		result["inbounds"] = inbounds
	}

	routing, ok := result["routing"].(map[string]interface{})
	if !ok {
		routing = map[string]interface{}{}
	}

	rules, ok := routing["rules"].([]interface{})
	if !ok {
		rules = []interface{}{}
	}

	hasAPIRule := false
	for _, rule := range rules {
		if r, ok := rule.(map[string]interface{}); ok {
			if outboundTag, ok := r["outboundTag"].(string); ok && outboundTag == "REMNAWAVE_API" {
				hasAPIRule = true
				break
			}
		}
	}

	if !hasAPIRule {
		apiRule := map[string]interface{}{
			"type":        "field",
			"outboundTag": "REMNAWAVE_API",
			"inboundTag":  []interface{}{"REMNAWAVE_API_INBOUND"},
		}
		rules = append([]interface{}{apiRule}, rules...)
		routing["rules"] = rules
		result["routing"] = routing
	}

	if _, ok := result["api"]; !ok {
		result["api"] = map[string]interface{}{
			"services": []interface{}{"HandlerService", "StatsService", "RoutingService"},
			"tag":      "REMNAWAVE_API",
		}
	} else {
		api, _ := result["api"].(map[string]interface{})
		if api != nil {
			services, _ := api["services"].([]interface{})
			hasRoutingService := false
			for _, s := range services {
				if str, ok := s.(string); ok && str == "RoutingService" {
					hasRoutingService = true
					break
				}
			}
			if !hasRoutingService {
				api["services"] = append(services, "RoutingService")
			}
		}
	}

	if _, ok := result["stats"]; !ok {
		result["stats"] = map[string]interface{}{}
	}

	outbounds, _ := result["outbounds"].([]interface{})
	hasBlockOutbound := false
	for _, ob := range outbounds {
		if outbound, ok := ob.(map[string]interface{}); ok {
			if tag, ok := outbound["tag"].(string); ok && tag == "BLOCK" {
				hasBlockOutbound = true
				break
			}
		}
	}
	if !hasBlockOutbound {
		blockOutbound := map[string]interface{}{
			"tag":      "BLOCK",
			"protocol": "blackhole",
			"settings": map[string]interface{}{
				"response": map[string]interface{}{
					"type": "http",
				},
			},
		}
		outbounds = append(outbounds, blockOutbound)
		result["outbounds"] = outbounds
	}

	existingPolicy, _ := result["policy"].(map[string]interface{})
	if existingPolicy == nil {
		existingPolicy = map[string]interface{}{}
	}

	existingLevels, _ := existingPolicy["levels"].(map[string]interface{})
	if existingLevels == nil {
		existingLevels = map[string]interface{}{}
	}

	existingLevel0, _ := existingLevels["0"].(map[string]interface{})
	if existingLevel0 == nil {
		existingLevel0 = map[string]interface{}{}
	}

	existingLevel0["statsUserUplink"] = true
	existingLevel0["statsUserDownlink"] = true
	existingLevel0["statsUserOnline"] = false

	existingLevels["0"] = existingLevel0
	existingPolicy["levels"] = existingLevels

	existingPolicy["system"] = map[string]interface{}{
		"statsInboundUplink":    true,
		"statsInboundDownlink":  true,
		"statsOutboundUplink":   true,
		"statsOutboundDownlink": true,
	}

	result["policy"] = existingPolicy

	return result
}
