package main

import "math/rand/v2"

type duelLine struct {
	Speaker string
	Text    string
}

var duelMemberLines = map[MemberID]map[string][]string{
	MemberTomori: {
		"duelStart": {"「一百万个格子……迷路的时候，也能一格一格继续前进。」"},
		"duelLead":  {"「大家一直在往前走呢……这一次不会再掉队了。」"},
		"duelAI":    {"「这颗小石头，先归我了……」"},
	},
	MemberAnon: {
		"duelStart":   {"「百万格超大舞台？这波抢地盘流量我吃定了！」"},
		"duelLead":    {"「占格数反超！百万格盘的绝对 C 位果然是我！」"},
		"duelBehind":  {"「等、等等！刚才只是鼠标飘了，马上追回来！」"},
		"duelReclaim": {"「这片格子贴上我的签名啦！承让了小灯~」"},
	},
	MemberSoyorin: {
		"duelStart": {"「偶尔不抢地盘……不行，今天请认真守住每一格呢。」"},
		"duelLead":  {"「步调很稳呢，就这样一点一点拉开差距吧。（微笑）」"},
		"duelMine":  {"「……为什么要去猜那一格呢？（失去高光）」"},
	},
	MemberTaki: {
		"duelStart":     {"「对手是灯我也不会放水……咳，快点开格！」"},
		"duelLead":      {"「对面可是灯，领先这点可别松懈了。」"},
		"duelBehind":    {"「喂！灯都推完半边了，你还在原地发呆吗？！」"},
		"duelWrongFlag": {"「这格明明安全你贴什么旗啊，数清楚再按！」"},
	},
	MemberRana: {
		"duelStart":   {"「好宽的草坪。我从这里开始。」"},
		"duelLead":    {"「连起来了，好听。像和弦一样喵。」"},
		"duelReclaim": {"「（叼走一块领地）……归我了。」"},
	},
}

func pickDuelLine(id MemberID, event string) (string, string) {
	if pool := duelMemberLines[id][event]; len(pool) > 0 {
		return FindMember(id).Name, pool[rand.IntN(len(pool))]
	}
	if event == "duelAI" {
		return "高松灯 (AI)", "「虽然很容易迷路，但我会一格一格好好记住的。」"
	}
	return "", ""
}
