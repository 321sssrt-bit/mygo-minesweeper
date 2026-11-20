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
	ID   MemberID
	Name string
	// Glyph is the one character on the member's avatar; Romaji follows
	// the name in small type.
	Glyph   string
	Romaji  string
	Role    string
	Tagline string
	// ColorHex is the member's official 代表色 (Image Color).
	ColorHex  string
	SkillName string
	SkillDesc string
	Passive   bool
}

var Members = []Member{
	{
		ID:        MemberTomori,
		Name:      "高松灯",
		Glyph:     "灯",
		Romaji:    "Tomori",
		Role:      "Vo.",
		Tagline:   "创可贴与石头收集家",
		ColorHex:  "#77BBDD",
		SkillName: "创可贴指引",
		SkillDesc: "经典：翻开安全边界；决斗：扫描鼠标周围 9×9 并解除硬直",
	},
	{
		ID:        MemberAnon,
		Name:      "千早爱音",
		Glyph:     "爱",
		Romaji:    "Anon",
		Role:      "Gt.",
		Tagline:   "虚荣SNS偶像担当",
		ColorHex:  "#FF8899",
		SkillName: "伦敦留学实力",
		SkillDesc: "经典：翻开两个安全格；决斗：扫描并把连击转为奖励分",
	},
	{
		ID:        MemberSoyorin,
		Name:      "长崎爽世",
		Glyph:     "爽",
		Romaji:    "Soyo",
		Role:      "Ba.",
		Tagline:   "重力微笑与拉黑之神",
		ColorHex:  "#FFDD88",
		SkillName: "温柔的微笑",
		SkillDesc: "被动：自动挡雷并收旗；决斗不结束比赛，但会扣除一次充能",
		Passive:   true,
	},
	{
		ID:        MemberTaki,
		Name:      "椎名立希",
		Glyph:     "立",
		Romaji:    "Taki",
		Role:      "Dr.",
		Tagline:   "压力怪 & 护灯狂魔",
		ColorHex:  "#7777AA",
		SkillName: "鼓棒指雷",
		SkillDesc: "经典：精准标雷；决斗：扫描并延迟 AI 下一次行动",
	},
	{
		ID:        MemberRana,
		Name:      "要乐奈",
		Glyph:     "乐",
		Romaji:    "Rāna",
		Role:      "Gt.",
		Tagline:   "野猫 & 抹茶芭菲爱好者",
		ColorHex:  "#77DD77",
		SkillName: "抹茶芭菲巡演",
		SkillDesc: "经典：展开空白区；决斗：扫描并延迟 AI 下一次行动",
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

const (
	MemeForever    = "forever"
	MemeHaruhikage = "haruhikage"
	MemeNeverHappy = "never_happy"
	MemeHuman      = "human"
	MemeMatcha     = "matcha"
	MemeAnonBest   = "anon_best"
)

type MemeAchievement struct {
	ID    string
	Title string
	Desc  string
}

var AllMemes = []MemeAchievement{
	{ID: MemeForever, Title: "一辈子！", Desc: "任意曲目完成排练，或在决斗中胜出"},
	{ID: MemeHaruhikage, Title: "为什么要演奏春日影！？", Desc: "在《春日影》决斗中比分输给高松灯"},
	{ID: MemeNeverHappy, Title: "我从来没有觉得扫雷开心过", Desc: "开局前 2 步就踩雷或触发护盾"},
	{ID: MemeHuman, Title: "好想成为人类啊", Desc: "由高松灯领队并发动技能后通关"},
	{ID: MemeMatcha, Title: "抹茶芭菲好吃喵", Desc: "用要乐奈的技能一口气连开 12 格以上"},
	{ID: MemeAnonBest, Title: "不愧是爱音大人", Desc: "不使用任何乐队技能完成一场排练"},
}

var memberQuotes = map[MemberID]map[string][]string{
	MemberTomori: {
		"start": {
			"「那个……可以和我组一辈子扫雷乐队吗？」",
			"「就算会迷路，也要一格一格地排下去……（攥紧小石头）」",
			"「今天也带了创可贴，遇到雷就先贴上吧。」",
		},
		"revealBig": {
			"「大家……都没有走散呢，前面的路一下子亮起来了。」",
			"「原来只要看着数字，就不会害怕迷路了。」",
		},
		"flag": {
			"「贴上创可贴盖好……这样就不会踩到了。」",
			"「这颗雷，我好好记在笔记本里了。」",
		},
		"skill": {
			"「这是我捡来的石头……这一格，应该没问题的。」",
		},
		"won": {
			"「太好了……这次我们一辈子都在一起了呢。」",
			"「大家都在这里，真是太好了……（拿出小石头）」",
		},
		"lost": {
			"「对不起……是不是因为我又把事情搞砸了……」",
			"「呜……如果刚才没有点那一格的话……」",
		},
		"haruhikageLost": {
			"「诶……为什么大家突然都停下来了？春日影……不能弹吗？」",
		},
	},
	MemberAnon: {
		"start": {
			"「这局就由我千早爱音来担任绝对C位！粉丝关注度UP！」",
			"「哼哼，我在伦敦留学时也是扫雷高手好吧~」",
			"「开局先点正中间！这可是SNS最流行的定式哦~」",
		},
		"revealBig": {
			"「哦哦哦！！直接连开一大片！快帮我截图发账号！」",
			"「这波操作简直天才！爱音酱大胜利~」",
		},
		"flag": {
			"「这颗雷已被爱音大人精准锁定！」",
			"「标旗完成~ 这种一眼雷完全难不倒我嘛。」",
		},
		"skill": {
			"「全员注目！看我用留学归来的双倍手速连开两格！」",
		},
		"won": {
			"「完美通关！果然没有我千早爱音 MyGO!!!!! 就不行吧~」",
			"「快把这局战绩设为置顶！一辈子都要夸我哦！」",
		},
		"lost": {
			"「诶诶诶？！怎么突然炸了啊！刚才绝对是鼠标飘了！」",
			"「不算不算！我还没准备好呢，快重开重开！」",
		},
		"haruhikageLost": {
			"「诶？！怎么弹完《春日影》，爽世世的脸突然黑成这样啊？！」",
		},
	},
	MemberSoyorin: {
		"start": {
			"「只要大家步调一致，这片雷区很快就能排完呢。（微笑）」",
			"「今天不会突然点到奇怪的曲子吧……？」",
			"「请大家不要急躁，好好确认每一个数字哦。」",
		},
		"revealBig": {
			"「配合得真好呢，这样下去大家就不会分开了。」",
			"「真是让人安心的展开呢。」",
		},
		"flag": {
			"「先把这颗雷贴好创可贴，免得待会儿吵起来呢。」",
			"「这里我就帮你记住啦。」",
		},
		"shield": {
			"「……刚才那一下，就当作没有点过吧。（微笑）」",
			"「没事的哦，这次我帮你挡下来了……下次不要再乱点了呢。」",
		},
		"won": {
			"「真是太好了呢，大家都平平安安的。（温柔微笑）」",
			"「辛苦了，接下来一起去买红茶吧。」",
		},
		"lost": {
			"「……为什么要去点那一格呢？（眼神失去高光）」",
			"「你这个人，满脑子都只想着瞎蒙呢。」",
		},
		"haruhikageLost": {
			"「为什么要演奏春日影？！……擅自选了这首曲子，结果还输掉了呢。」",
		},
	},
	MemberTaki: {
		"start": {
			"「哈？！连这种基础定式都推不出来吗？看着点数字！」",
			"「别磨蹭了，跟上我的鼓点把这局拿下。」",
			"「喂，要开局就认真点，别连累到灯。」",
		},
		"revealBig": {
			"「切，算你手感不错，连开了一大片。」",
			"「节奏还可以，别在最后关头掉链子就行。」",
		},
		"flag": {
			"「这种一眼雷还不赶紧标上？动作快点！」",
			"「标好了，继续往旁边推。」",
		},
		"skill": {
			"「啧，连这颗雷都要我用鼓棒敲出来吗？拿去，标好了！」",
		},
		"won": {
			"「哼，勉强算合格吧。灯，水拿好了。」",
			"「总算搞定了。下一把要不要直接挑战春日影？」",
		},
		"lost": {
			"「喂！！你在按什么啊！！数字 1 旁边有几个雷看不见吗？！」",
			"「哈？！能确定的格子不去开，非要去瞎猜？！」",
		},
		"haruhikageLost": {
			"「喂！都说了别乱弹《春日影》啊！！输掉不说，你看爽世脸都黑了！！」",
		},
	},
	MemberRana: {
		"start": {
			"「抹茶芭菲……这里好像有好玩的格子。」",
			"「（抱着吉他蹲在棋盘角落）……有趣。」",
			"「喵？要开始弹了吗？」",
		},
		"revealBig": {
			"「哦，哗啦一下全开了。像吉他滑音一样。」",
			"「有趣的格子……还想再点。」",
		},
		"flag": {
			"「（叼着小黄瓜标了一格）……这里不行喵。」",
			"「有危险的味道，不吃。」",
		},
		"skill": {
			"「随性巡演开始——（空降一片 0 雷空白区）」",
		},
		"won": {
			"「排完了？那我去 RiNG 吃抹茶芭菲了。」",
			"「有趣的女人……哦不，有趣的扫雷。」",
		},
		"lost": {
			"「没意思，走了。（背起吉他离开现场）」",
			"「轰的一声……吉他音箱炸掉了喵？」",
		},
		"haruhikageLost": {
			"「春日影？有趣。（弹完就溜，留下一片混乱）」",
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
		pool = events["lost"]
	}
	if len(pool) == 0 {
		return "「迷子でもいい、迷子でも進め。」"
	}
	return pool[rand.IntN(len(pool))]
}
