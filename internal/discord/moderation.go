package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"discord-agent/internal/config"
	"discord-agent/internal/llm"
	"discord-agent/internal/tools"
	"discord-agent/internal/util"

	"github.com/bwmarrin/discordgo"
)

func RegisterModerationTools(reg *tools.Registry, b *Bot) {
	pendingTool := func(name, desc string, kind string, minutes bool) {
		props := map[string]any{
			"user_id": map[string]any{"type": "string", "description": "目標使用者 ID"},
			"reason":  map[string]any{"type": "string", "description": "原因，可簡短"},
		}
		required := []string{"user_id"}
		if minutes {
			props["minutes"] = map[string]any{"type": "integer", "description": "靜音分鐘數，預設 60，最多 10080"}
		}
		reg.AddTool(llm.ToolDef{
			Name: name, Description: desc,
			Parameters: map[string]any{"type": "object", "properties": props, "required": required},
		}, func(ctx context.Context, args json.RawMessage) (string, error) {
			ref, err := requireAdmin(ctx)
			if err != nil {
				return "", err
			}
			var in struct {
				UserID  string `json:"user_id"`
				Reason  string `json:"reason"`
				Minutes int    `json:"minutes"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			if strings.TrimSpace(in.UserID) == "" {
				return "", fmt.Errorf("user_id 不可為空")
			}
			if in.Minutes < 1 {
				in.Minutes = 60
			}
			if in.Minutes > 10080 {
				in.Minutes = 10080
			}
			a := PendingAction{
				ID: config.NewID("act_"), Type: kind, UserID: in.UserID,
				GuildID: ref.GuildID, Reason: in.Reason, Minutes: in.Minutes,
				ChannelID: ref.ChannelID,
			}
			b.pendingMu.Lock()
			b.pendingActions[a.ID] = a
			b.pendingMu.Unlock()
			addPending(ctx, a)
			return "已建立待確認的「" + kind + "」動作（" + a.ID + "）。請用一般語氣向使用者說明原因，按確定後才會執行。", nil
		})
	}
	pendingTool("ban_user", "申請封鎖使用者。需管理員按確定後才會執行。", "ban", false)
	pendingTool("kick_user", "申請踢出使用者。需管理員按確定後才會執行。", "kick", false)
	pendingTool("mute_user", "申請靜音使用者。需管理員按確定後才會執行。", "mute", true)

	reg.AddTool(llm.ToolDef{
		Name:        "read_audit_log",
		Description: "讀取伺服器管理日誌（稽核日誌）。僅管理員可用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit":       map[string]any{"type": "integer", "description": "筆數，最多 50"},
				"action_type": map[string]any{"type": "integer", "description": "Discord audit log action type，可省略"},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, err := requireAdmin(ctx)
		if err != nil {
			return "", err
		}
		var in struct {
			Limit      int `json:"limit"`
			ActionType int `json:"action_type"`
		}
		if len(args) > 0 && string(args) != "null" {
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
		}
		if in.Limit < 1 || in.Limit > 50 {
			in.Limit = 20
		}
		log, err := b.session.GuildAuditLog(ref.GuildID, "", "", in.ActionType, in.Limit)
		if err != nil {
			return "", err
		}
		var lines []string
		for _, e := range log.AuditLogEntries {
			action := ""
			if e.ActionType != nil {
				action = strconv.Itoa(int(*e.ActionType))
			}
			lines = append(lines, fmt.Sprintf("[%s] 操作者:%s 目標:%s 原因:%s", action, e.UserID, e.TargetID, e.Reason))
		}
		if len(lines) == 0 {
			return "沒有稽核日誌紀錄。", nil
		}
		return strings.Join(lines, "\n"), nil
	})

	reg.AddTool(llm.ToolDef{
		Name: "get_member_info", Description: "取得伺服器成員資訊。僅管理員可用。",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"user_id": map[string]any{"type": "string"},
			}, "required": []string{"user_id"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		ref, err := requireAdmin(ctx)
		if err != nil {
			return "", err
		}
		var in struct {
			UserID string `json:"user_id"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return "", err
		}
		m, err := b.session.GuildMember(ref.GuildID, in.UserID)
		if err != nil {
			return "", err
		}
		name := ""
		if m.User != nil {
			name = m.User.Username
		}
		return fmt.Sprintf("使用者：%s\nID：%s\n加入時間：%s", name, in.UserID, m.JoinedAt), nil
	})

	reg.AddTool(llm.ToolDef{
		Name: "list_channels", Description: "列出伺服器頻道。僅管理員可用。",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(ctx context.Context, _ json.RawMessage) (string, error) {
		ref, err := requireAdmin(ctx)
		if err != nil {
			return "", err
		}
		channels, err := b.session.GuildChannels(ref.GuildID)
		if err != nil {
			return "", err
		}
		var lines []string
		for _, c := range channels {
			lines = append(lines, fmt.Sprintf("#%s (%v) %s", c.Name, c.Type, c.ID))
		}
		return strings.Join(lines, "\n"), nil
	})

	reg.AddTool(llm.ToolDef{
		Name: "get_guild_info", Description: "取得伺服器基本資訊。僅管理員可用。",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
	}, func(ctx context.Context, _ json.RawMessage) (string, error) {
		ref, err := requireAdmin(ctx)
		if err != nil {
			return "", err
		}
		g, err := b.session.Guild(ref.GuildID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("伺服器：%s\nID：%s\n成員數：%d", g.Name, g.ID, g.MemberCount), nil
	})
}

func requireAdmin(ctx context.Context) (MessageRef, error) {
	ref, ok := messageRefFrom(ctx)
	if !ok || ref.GuildID == "" {
		return ref, fmt.Errorf("此工具只能在伺服器的文字頻道使用")
	}
	if !ref.IsAdmin {
		return ref, fmt.Errorf("權限不足：僅管理員可使用此工具")
	}
	return ref, nil
}

func (b *Bot) sendPendingButtons(s *discordgo.Session, channelID string, items []PendingAction) {
	for _, a := range items {
		components := []discordgo.MessageComponent{discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "確定", Style: discordgo.SuccessButton, CustomID: "mod_confirm_" + a.ID},
				discordgo.Button{Label: "取消", Style: discordgo.DangerButton, CustomID: "mod_cancel_" + a.ID},
			},
		}}
		content := "請管理員確認：" + a.Type + " <@" + a.UserID + ">（僅管理員可按）"
		if _, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content: content, Components: components, AllowedMentions: allowedUserMentions(),
		}); err != nil {
			b.log.Printf("傳送確認按鈕失敗: %s", util.Redact(err.Error(), b.secrets()))
		}
	}
}

func (b *Bot) onComponent(s *discordgo.Session, ev *discordgo.InteractionCreate) {
	i := ev.Interaction
	data, ok := i.Data.(discordgo.MessageComponentInteractionData)
	if !ok {
		return
	}
	if strings.HasPrefix(data.CustomID, "ui_btn_") || strings.HasPrefix(data.CustomID, "ui_sel_") {
		b.handleUIComponent(s, ev, data)
		return
	}
	userID := ""
	if i.User != nil {
		userID = i.User.ID
	} else if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	}
	cfg := b.store.Get()
	if !contains(cfg.Discord.AdminUserIDs, userID) {
		_ = s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "權限不足：僅管理員可按。", Flags: discordgo.MessageFlagsEphemeral},
		})
		return
	}
	id := ""
	confirm := false
	switch {
	case strings.HasPrefix(data.CustomID, "mod_confirm_"):
		id, confirm = strings.TrimPrefix(data.CustomID, "mod_confirm_"), true
	case strings.HasPrefix(data.CustomID, "mod_cancel_"):
		id = strings.TrimPrefix(data.CustomID, "mod_cancel_")
	default:
		return
	}
	b.pendingMu.Lock()
	action, ok := b.pendingActions[id]
	delete(b.pendingActions, id)
	b.pendingMu.Unlock()
	if !ok {
		_ = s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "這項動作已失效。", Flags: discordgo.MessageFlagsEphemeral},
		})
		return
	}
	result := "已取消。"
	if confirm {
		result = b.executeAction(action)
	}
	content := result
	components := []discordgo.MessageComponent{}
	_ = s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{Content: content, Components: components},
	})
}

func (b *Bot) executeAction(a PendingAction) string {
	var err error
	switch a.Type {
	case "ban":
		err = b.session.GuildBanCreateWithReason(a.GuildID, a.UserID, a.Reason, 1)
	case "kick":
		err = b.session.GuildMemberDeleteWithReason(a.GuildID, a.UserID, a.Reason)
	case "mute":
		until := time.Now().Add(time.Duration(a.Minutes) * time.Minute)
		err = b.session.GuildMemberTimeout(a.GuildID, a.UserID, &until)
	default:
		return "未知動作。"
	}
	if err != nil {
		b.log.Printf("執行管理動作失敗: %s", util.Redact(err.Error(), b.secrets()))
		return "執行失敗，請檢查權限。"
	}
	return "已執行：" + a.Type
}
