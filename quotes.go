package main

import "math/rand/v2"

type MemberID string

const (
	MemberTomori  MemberID = "tomori"
	MemberAnon    MemberID = "anon"
	MemberSoyorin MemberID = "soyorin"
	MemberTaki    MemberID = "taki"
	MemberRana    MemberID = "rana"
)

type Member struct {
	ID        MemberID
	Name      string
	Role      string
	ColorHex  string
	SkillName string
	SkillDesc string
	Passive   bool
}

var Members = []Member{
	{
		ID:        MemberTomori,
		Name:      "高松灯",
		Role:      "Vo.",
		ColorHex:  "#67B7E1",
		SkillName: "捡来的石头",
		SkillDesc: "主动：安全翻开 1 个非地雷格",
	},
	{
		ID:        MemberAnon,
		Name:      "千早爱音",
		Role:      "Gt.",
		ColorHex:  "#F07AA4",
		SkillName: "SNS 高光",
		SkillDesc: "主动：同时翻开最多 2 个安全格",
	},
	{
		ID:        MemberSoyorin,
		Name:      "长崎爽世",
		Role:      "Ba.",
		ColorHex:  "#E8A849",
		SkillName: "温柔的微笑",
		SkillDesc: "被动：误触地雷时自动免疫并插旗",
		Passive:   true,
	},
	{
		ID:        MemberTaki,
		Name:      "椎名立希",
		Role:      "Dr.",
		ColorHex:  "#9B7CE8",
		SkillName: "精准节拍",
		SkillDesc: "主动：自动标记 1 个未标旗的地雷",
	},
	{
		ID:        MemberRana,
		Name:      "要乐奈",
		Role:      "Gt.",
		ColorHex:  "#68C178",
		SkillName: "随性巡演",
		SkillDesc: "主动：直接翻开 1 个空白连锁区域",
	},
}

func FindMember(id MemberID) Member {
	for _, m := range Members {
		if m.ID == id {
			return m
		}
	}
	return Members[0]
}

var memberQuotes = map[MemberID]map[string][]string{
	MemberTomori: {
		"start": {
			"「就算会迷路……也想和大家一起把这片雷区走完。」",
			"「每一块石头底下，说不定都有不一样的心情。」",
			"「没关系的，慢慢确认每一个数字吧。」",
		},
		"revealBig": {
			"「哇……前面一下子变得好开阔。」",
			"「顺着这些数字走，就不会迷失方向了。」",
		},
		"flag": {
			"「贴上创可贴标记好……这样就不会踩到了。」",
			"「这里很危险，要好好记住。」",
		},
		"skill": {
			"「这是我捡来的石头……这一格，应该是安全的。」",
		},
		"won": {
			"「太好了……我们谁都没有走散呢。一辈子一起扫雷吧。」",
			"「大家都在这里，真是太好了。」",
		},
		"lost": {
			"「对不起……都是因为我没看清那里的数字吗……」",
			"「是不是又把事情搞砸了……」",
		},
	},
	MemberAnon: {
		"start": {
			"「这局就由我千早爱音来担任C位扫雷手！关注度UP！」",
			"「轻轻松松啦，这种推理游戏我可是很有自信的~」",
			"「伦敦留学经验告诉我，开局先点中间准没错！」",
		},
		"revealBig": {
			"「哦哦哦！这波直接连开一片，快截图发SNS！」",
			"「不愧是我，节奏感拉满了！」",
		},
		"flag": {
			"「哼哼，这颗雷已经被我完美锁定啦。」",
			"「危险标志贴上！爱音大人可不会上当~」",
		},
		"skill": {
			"「全员注目！看我用SNS高光雷达连开两格！」",
		},
		"won": {
			"「完美通关！果然有我在，MyGO!!!!!就是最强！」",
			"「快看计时器！这波操作绝对能涨粉吧~」",
		},
		"lost": {
			"「诶诶诶？！怎么突然就爆了啊喂！刚才明明算好的！」",
			"「不算不算！这局肯定是我手指滑了，重开重开！」",
		},
	},
	MemberSoyorin: {
		"start": {
			"「只要大家保持步调一致，这片雷区很快就能排完吧。」",
			"「请大家不要急躁，安安静静地确认每一格呢。」",
			"「……今天不会演奏那首曲子吧？」",
		},
		"revealBig": {
			"「真是顺利呢，这样下去很快就能结束了。」",
			"「配合得很不错哦，继续保持吧。」",
		},
		"flag": {
			"「把危险的地方先标记起来，这样大家才安心。」",
			"「这里我就先帮忙记下来了哦。」",
		},
		"shield": {
			"「……刚才那一下，就当作没有发生过吧。（微笑）」",
			"「没事的哦，这次我帮你挡下来了。下次小心一点呢。」",
		},
		"won": {
			"「真是太好了呢，大家都平平安安的。（微笑）」",
			"「辛苦了，接下来一起去喝茶吧。」",
		},
		"lost": {
			"「……为什么要去点那一格呢？（眼神失去高光）」",
			"「为什么要演奏春日影……我是说，为什么要盲猜这一格？！」",
		},
	},
	MemberTaki: {
		"start": {
			"「哈？连这点基础定式都推不出来吗？看着点数字！」",
			"「别磨蹭了，跟上我的节拍把这局拿下。」",
			"「行了，要开局就认真点，别连累灯。」",
		},
		"revealBig": {
			"「切，算你运气好，连开了一大片。」",
			"「节奏不错，别在最后关头掉链子就行。」",
		},
		"flag": {
			"「这种一眼雷还不赶紧标上？动作快点。」",
			"「标好了，继续推下一片。」",
		},
		"skill": {
			"「啧，连这颗雷都要我提醒吗？拿去，标好了！」",
		},
		"won": {
			"「哼，勉强算合格吧。灯，水给你。」",
			"「搞定了。下一把要不要直接上春日影难度？」",
		},
		"lost": {
			"「喂！你在按什么啊！！数字1旁边两个雷都看不见吗？！」",
			"「哈？！明明能确定的地方不去推，偏要瞎蒙？！」",
		},
	},
	MemberRana: {
		"start": {
			"「抹茶芭菲……这里好像有好玩的格子。」",
			"「（抱着吉他蹲在雷区旁边）……有趣。」",
			"「喵？开始了？」",
		},
		"revealBig": {
			"「哦，一下子全开了。有趣。」",
			"「像吉他的滑音一样，哗啦一下全开了。」",
		},
		"flag": {
			"「（用小黄瓜标记了一格）……这里不行。」",
			"「喵，有雷的味道。」",
		},
		"skill": {
			"「随性巡演开始——（直接弹向一片安全区）」",
		},
		"won": {
			"「排完了？那我去吃抹茶芭菲了。」",
			"「有趣的女人……哦不，有趣的扫雷。」",
		},
		"lost": {
			"「没意思，走了。（背起吉他离开现场）」",
			"「轰的一声……吉他音箱爆掉了？」",
		},
	},
}

func PickQuote(memberID MemberID, event string) string {
	events, ok := memberQuotes[memberID]
	if !ok {
		events = memberQuotes[MemberTomori]
	}
	pool := events[event]
	if len(pool) == 0 {
		return "「迷子でもいい、迷子でも進め。」"
	}
	return pool[rand.IntN(len(pool))]
}
