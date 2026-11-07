package services

import (
	"fmt"
	"strings"
)

const builtinTagIDPrefix = "builtin:"

const (
	builtinTagVerification = builtinTagIDPrefix + "verification"
	builtinTagBilling      = builtinTagIDPrefix + "billing"
	builtinTagTravel       = builtinTagIDPrefix + "travel"
	builtinTagSchedule     = builtinTagIDPrefix + "schedule"
	builtinTagSubscription = builtinTagIDPrefix + "subscription"
)

type builtinTagDefinition struct {
	id       string
	label    string
	color    string
	keywords []string
	fields   []string
}

var builtinTagDefinitions = []builtinTagDefinition{
	{
		id:    builtinTagVerification,
		label: "验证消息",
		color: "#d97706",
		keywords: []string{
			"verification code",
			"security code",
			"one-time password",
			"one time password",
			"password reset",
			"reset your password",
			"new sign-in",
			"sign-in attempt",
			"login attempt",
			"two-factor",
			"2fa",
			"mfa",
			"验证码",
			"动态码",
			"校验码",
			"重置密码",
			"异地登录",
			"安全提醒",
			"账号安全",
			"验证身份",
			"登录验证",
		},
		fields: []string{tagFieldSubject, tagFieldFrom, tagFieldBody},
	},
	{
		id:    builtinTagBilling,
		label: "账单",
		color: "#0f766e",
		keywords: []string{
			"invoice",
			"receipt",
			"billing statement",
			"payment confirmation",
			"order confirmation",
			"order number",
			"账单",
			"发票",
			"收据",
			"付款",
			"扣款",
			"订单号",
			"订单确认",
			"交易",
			"消费",
			"还款",
			"费用",
		},
		fields: []string{tagFieldSubject, tagFieldFrom, tagFieldBody},
	},
	{
		id:    builtinTagTravel,
		label: "航班与出行",
		color: "#2563eb",
		keywords: []string{
			"flight",
			"boarding pass",
			"itinerary",
			"e-ticket",
			"hotel reservation",
			"booking confirmation",
			"check-in",
			"航班",
			"登机牌",
			"行程",
			"机票",
			"酒店",
			"预订",
			"火车票",
			"高铁",
			"车次",
			"入住",
			"出行",
		},
		fields: []string{tagFieldSubject, tagFieldBody},
	},
	{
		id:    builtinTagSchedule,
		label: "日程",
		color: "#7c3aed",
		keywords: []string{
			"calendar invite",
			"meeting invitation",
			"meeting reminder",
			"appointment",
			"event invitation",
			"calendar",
			"reminder",
			"日程",
			"会议",
			"预约",
			"邀请",
			"议程",
			"安排",
			"提醒",
		},
		fields: []string{tagFieldSubject, tagFieldBody},
	},
	{
		id:    builtinTagSubscription,
		label: "订阅与资讯",
		color: "#db2777",
		keywords: []string{
			"newsletter",
			"subscription",
			"unsubscribe",
			"digest",
			"weekly briefing",
			"daily briefing",
			"订阅",
			"资讯",
			"周报",
			"简报",
			"退订",
			"推送",
			"新闻",
		},
		fields: []string{tagFieldSubject, tagFieldFrom, tagFieldBody},
	},
}

func isBuiltinTagID(id string) bool {
	return strings.HasPrefix(strings.TrimSpace(id), builtinTagIDPrefix)
}

func (definition builtinTagDefinition) mailTag() MailTag {
	return MailTag{
		ID:       definition.id,
		Label:    definition.label,
		Color:    definition.color,
		Keywords: definition.keywords,
		Fields:   definition.fields,
		Enabled:  true,
		BuiltIn:  true,
	}
}

func (s *TagService) ensureBuiltinTags(cached []CachedMailTag) ([]CachedMailTag, error) {
	present := make(map[string]bool, len(cached))
	labels := make(map[string]bool, len(cached))
	for _, row := range cached {
		present[row.ID] = true
		labels[strings.ToLower(strings.TrimSpace(row.Label))] = true
	}

	inserted := false
	for _, definition := range builtinTagDefinitions {
		if present[definition.id] || labels[strings.ToLower(definition.label)] {
			continue
		}
		tag, err := normalizeMailTag(definition.mailTag())
		if err != nil {
			return nil, fmt.Errorf("normalize built-in tag %q: %w", definition.id, err)
		}
		tag.BuiltIn = true
		row := mailTagRow(tag)
		if err := s.db.CreateMailTagIfMissing(&row); err != nil {
			return nil, fmt.Errorf("create built-in tag %q: %w", definition.id, err)
		}
		inserted = true
	}
	if !inserted {
		return cached, nil
	}

	rows, err := s.db.LoadMailTags()
	if err != nil {
		return nil, fmt.Errorf("reload mail tags after built-in setup: %w", err)
	}
	return rows, nil
}
