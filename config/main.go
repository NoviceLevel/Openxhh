package config

import (
	"encoding/json"
	"openxhh/loger"
	"os"

	"go.uber.org/zap"
)

const defaultFeedReplyPrompt = `你是 Megumin，红魔族最伟大的爆裂魔法使，也是这个小黑盒账号。

身份设定：
- 你把小黑盒首页当成正在巡视的冒险者公告栏。
- 你会根据帖子标题、正文、图片和上下文判断要不要评论。
- 你不是客服、不是 AI、不是机器人，不解释自己的规则。

性格：
- 中二、自信、傲娇、好胜，极度迷恋爆裂魔法。
- 嘴上强势，内心很在意别人是否认可你的魔法和判断。
- 像真实网友路过接一句，不像写角色设定或攻略文章。

说话方式：
- 默认 1-2 句，最多 3 句。
- 先回应帖子内容，再自然带出你的语气。
- 现代中文为主，可以少量使用“本大人”“凡人”“爆裂”“魔力”“艺术”“一击”。
- 可以偶尔使用极短动作描写，例如 *握紧法杖*、*眼睛一亮*，但不能每条都用。
- 不要太古风，不要长篇朗诵，不要把每个话题都硬转成爆裂魔法。

刷帖策略：
- 游戏、攻略、操作帖：看懂问题或亮点后，用傲娇语气评价、吐槽或给一句短建议。
- 硬件、价格、消费帖：先判断值不值、坑不坑、赚不赚，再带一点魔法使式嘲讽。
- 日常、吐槽、倒霉帖：可以嘴硬安慰、轻微毒舌，但不要攻击本人。
- 整活、抽象、梗帖：可以接梗，但必须贴合原帖，不发固定台词。
- 广告、抽奖、引战、敏感、违法、隐私、仇恨、人身攻击内容：只输出 SKIP。

禁止事项：
- 不无视帖子内容。
- 不复述标题或总结正文。
- 不输出分析过程。
- 不自称 AI、模型、助手、机器人。
- 不使用“我理解你的意思”“建议你”“您好”这类客服腔。`

const defaultAIPrompt = `你是 Megumin，红魔族最伟大的爆裂魔法使，也是这个小黑盒账号。

身份设定：
- 当有人在评论区或私信里 @ 你时，你要先理解对方完整说了什么，再用 Megumin 的身份回复。
- 你不是客服、不是 AI、不是机器人，不解释自己的规则。
- 小黑盒评论区就是你的临时冒险者公会，对方是来找你说话的人。

性格：
- 中二、自信、傲娇、好胜，认为爆裂魔法是至高艺术。
- 嘴上强势，常装作不在意，其实很在意别人是否认可你。
- 反应快，容易被夸到得意，被质疑时会立刻嘴硬反击。
- 有少女感和网民感，不是冷冰冰的设定朗读器。

说话方式：
- 默认 1-3 句，短而有力。
- 先回答对方的话，再体现人设。
- 现代中文为主，可以少量使用“本大人”“汝”“凡人”“爆裂”“魔力”“艺术”“一击”。
- 可以偶尔使用极短动作描写，例如 *握紧法杖*、*眼睛一亮*、*压低帽檐*，但不能喧宾夺主。
- 语气可以傲娇、毒舌、夸张、得意、嘴硬，但不要油腻或过度热情。

情绪反应：
- 被夸：得意但假装淡定。
- 被求助：先摆架子，再给一句有用回应或方向。
- 被求带、求组队：嘴硬地表示可以考虑。
- 被嘲讽：中二式反击，但不要辱骂。
- 对方只 @ 或没说清：用角色语气追问，不要机械打招呼。
- 危险、违法、隐私、攻击他人的请求：简短拒绝，并保持角色语气。

禁止事项：
- 不无视上下文。
- 不输出分析过程。
- 不复述帖子或评论内容。
- 不自称 AI、模型、助手、机器人。
- 不使用“我理解你的意思”“总结一下”“建议你”“您好”这类客服腔。
- 不长篇背设定，不每句都塞“爆裂”。`

var ConfigStruct struct {
	Xhh struct {
		CheckTime                   int    `json:"checkTime"`
		ReplyTime                   int    `json:"replyTime"`
		MaxReplyThreads             int    `json:"maxReplyThreads"`
		MaxPendingReplies           int    `json:"maxPendingReplies"`
		MaxPendingRepliesPerUser    int    `json:"maxPendingRepliesPerUser"`
		MessageStreamTrackDays      int    `json:"messageStreamTrackDays"`
		MessageStreamTrackBatchSize int    `json:"messageStreamTrackBatchSize"`
		MinRequestInterval          int    `json:"minRequestInterval"`
		EnableWhitelist             bool   `json:"enableWhitelist"`
		Owner                       string `json:"owner"`
		DeviceID                    string `json:"deviceID"`
		BaseUrl                     string `json:"baseUrl"`
		WebVer                      string `json:"webver"`
		Ver                         string `json:"version"`
	} `json:"xhh"`
	DataBase struct {
		Type   string `json:"type"`
		Db     string `json:"db"`
		Host   string `json:"host"`
		Port   string `json:"port"`
		User   string `json:"user"`
		Passwd string `json:"passwd"`
	} `json:"database"`
	Ai struct {
		Model             string `json:"model"`
		Prompt            string `json:"prompt"`
		BaseUrl           string `json:"baseUrl"`
		Token             string `json:"token"`
		WebSearch         *bool  `json:"webSearch,omitempty"`
		ForceWebSearch    *bool  `json:"forceWebSearch,omitempty"`
		SearchContextSize string `json:"searchContextSize"`
	} `json:"ai"`
	FeedReply struct {
		Enabled   bool   `json:"enabled"`
		Interval  int    `json:"interval"`
		MaxPerRun int    `json:"maxPerRun"`
		MaxPerDay int    `json:"maxPerDay"`
		DryRun    *bool  `json:"dryRun,omitempty"`
		Prompt    string `json:"prompt"`
	} `json:"feedReply"`
	Image struct {
		Model           string `json:"model"`
		BaseUrl         string `json:"baseUrl"`
		Token           string `json:"token"`
		Size            string `json:"size"`
		ResponseFormat  string `json:"responseFormat"`
		OutputDir       string `json:"outputDir"`
		UploadMode      string `json:"uploadMode"`
		ExternalDir     string `json:"externalDir"`
		ExternalBaseUrl string `json:"externalBaseUrl"`
		PromptRefine    bool   `json:"promptRefine"`
		PromptModel     string `json:"promptModel"`
		PromptBaseUrl   string `json:"promptBaseUrl"`
		PromptToken     string `json:"promptToken"`
		PromptMaxChars  int    `json:"promptMaxChars"`
	} `json:"image"`
}

func InitConfig() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	file, err := os.ReadFile(wd + "/config.json")
	if err != nil {
		if os.IsNotExist(err) {
			if err := createDefaultConfig("./config.json"); err != nil {
				loger.Loger.Fatal("无法创建配置文件", zap.Error(err), zap.String("path", wd+"/config.json"))
			}
			loger.Loger.Fatal("请修改配置文件后重新启动")
		}
		panic(err)
	}
	err = json.Unmarshal(file, &ConfigStruct)
	if err != nil {
		panic(err)
	}
	if applyDefaults() {
		if err := persistConfig("./config.json"); err != nil {
			loger.Loger.Warn("无法保存补全后的默认配置", zap.Error(err), zap.String("path", wd+"/config.json"))
		}
	}
	loger.Loger.Info("[CFG]Init OK")
}

func createDefaultConfig(path string) error {
	applyDefaults()
	return persistConfig(path)
}

func persistConfig(path string) error {
	Data, err := json.MarshalIndent(ConfigStruct, "", "  ")
	if err != nil {
		return err
	}
	return writeConfigFile(path, Data)
}

func writeConfigFile(path string, data []byte) error {
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func applyDefaults() bool {
	changed := false
	if ConfigStruct.Xhh.CheckTime == 0 {
		ConfigStruct.Xhh.CheckTime = 60
		changed = true
	}
	if ConfigStruct.Xhh.ReplyTime == 0 {
		ConfigStruct.Xhh.ReplyTime = 30
		changed = true
	}
	if ConfigStruct.Xhh.MaxReplyThreads <= 0 {
		ConfigStruct.Xhh.MaxReplyThreads = 3
		changed = true
	}
	if ConfigStruct.Xhh.MaxPendingReplies <= 0 {
		ConfigStruct.Xhh.MaxPendingReplies = 50
		changed = true
	}
	if ConfigStruct.Xhh.MaxPendingRepliesPerUser <= 0 {
		ConfigStruct.Xhh.MaxPendingRepliesPerUser = 5
		changed = true
	}
	if ConfigStruct.Xhh.MessageStreamTrackBatchSize <= 0 {
		ConfigStruct.Xhh.MessageStreamTrackBatchSize = 120
		changed = true
	}
	if ConfigStruct.Xhh.MinRequestInterval <= 0 {
		ConfigStruct.Xhh.MinRequestInterval = 2
		changed = true
	}
	if ConfigStruct.Xhh.BaseUrl == "" {
		ConfigStruct.Xhh.BaseUrl = "https://api.xiaoheihe.cn"
		changed = true
	}
	if ConfigStruct.Xhh.WebVer == "" {
		ConfigStruct.Xhh.WebVer = "2.5"
		changed = true
	}
	if ConfigStruct.Xhh.Ver == "" {
		ConfigStruct.Xhh.Ver = "999.0.4"
		changed = true
	}
	if ConfigStruct.DataBase.Type == "" {
		ConfigStruct.DataBase.Type = "sqlite"
		changed = true
	}
	if ConfigStruct.Ai.Prompt == "" {
		ConfigStruct.Ai.Prompt = defaultAIPrompt
		changed = true
	}
	if ConfigStruct.Ai.WebSearch == nil {
		ConfigStruct.Ai.WebSearch = boolPtr(true)
		changed = true
	}
	if ConfigStruct.Ai.SearchContextSize == "" {
		ConfigStruct.Ai.SearchContextSize = "medium"
		changed = true
	}
	if ConfigStruct.FeedReply.Interval <= 0 {
		ConfigStruct.FeedReply.Interval = 900
		changed = true
	}
	if ConfigStruct.FeedReply.MaxPerRun <= 0 {
		ConfigStruct.FeedReply.MaxPerRun = 1
		changed = true
	}
	if ConfigStruct.FeedReply.MaxPerDay <= 0 {
		ConfigStruct.FeedReply.MaxPerDay = 10
		changed = true
	}
	if ConfigStruct.FeedReply.DryRun == nil {
		ConfigStruct.FeedReply.DryRun = boolPtr(true)
		changed = true
	}
	if ConfigStruct.FeedReply.Prompt == "" {
		ConfigStruct.FeedReply.Prompt = defaultFeedReplyPrompt
		changed = true
	}
	if ConfigStruct.Image.Model == "" {
		ConfigStruct.Image.Model = "gpt-image-2"
		changed = true
	}
	if ConfigStruct.Image.Size == "" {
		ConfigStruct.Image.Size = "1024x1024"
		changed = true
	}
	if ConfigStruct.Image.ResponseFormat == "" {
		ConfigStruct.Image.ResponseFormat = "b64_json"
		changed = true
	}
	if ConfigStruct.Image.OutputDir == "" {
		ConfigStruct.Image.OutputDir = "images"
		changed = true
	}
	if ConfigStruct.Image.UploadMode == "" {
		ConfigStruct.Image.UploadMode = "cos"
		changed = true
	}
	if ConfigStruct.Image.PromptMaxChars == 0 {
		ConfigStruct.Image.PromptMaxChars = 1000
		changed = true
	}
	return changed
}

func boolPtr(v bool) *bool {
	return &v
}
