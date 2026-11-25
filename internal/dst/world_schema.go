package dst

type WorldOptionChoice struct {
	Value   string `json:"value"`
	LabelZH string `json:"label_zh"`
	LabelEN string `json:"label_en"`
}

type WorldOptionField struct {
	Key     string              `json:"key"`
	LabelZH string              `json:"label_zh"`
	LabelEN string              `json:"label_en"`
	Default string              `json:"default"`
	Order   int                 `json:"order,omitempty"`
	Choices []WorldOptionChoice `json:"choices"`
}

type WorldOptionGroup struct {
	ID      string             `json:"id"`
	Section string             `json:"section"` // "settings" (World Settings) or "worldgen" (World Generation)
	NameZH  string             `json:"name_zh"`
	NameEN  string             `json:"name_en"`
	Options []WorldOptionField `json:"options"`
}

var (
	// World Settings frequency_descriptions (5 levels)
	freqChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "无", LabelEN: "None"},
		{Value: "rare", LabelZH: "很少", LabelEN: "Little"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "often", LabelZH: "较多", LabelEN: "More"},
		{Value: "always", LabelZH: "大量", LabelEN: "Tons"},
	}

	// World Generation worldgen_frequency_descriptions (8 levels, PC/non-console)
	worldgenFreqChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "无", LabelEN: "None"},
		{Value: "rare", LabelZH: "很少", LabelEN: "Little"},
		{Value: "uncommon", LabelZH: "较少", LabelEN: "Less"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "often", LabelZH: "较多", LabelEN: "More"},
		{Value: "mostly", LabelZH: "很多", LabelEN: "Lots"},
		{Value: "always", LabelZH: "大量", LabelEN: "Tons"},
		{Value: "insane", LabelZH: "疯狂", LabelEN: "Insane"},
	}

	// ocean_worldgen_frequency_descriptions (8 levels with ocean_ prefix)
	oceanWorldgenFreqChoices = []WorldOptionChoice{
		{Value: "ocean_never", LabelZH: "无", LabelEN: "None"},
		{Value: "ocean_rare", LabelZH: "很少", LabelEN: "Little"},
		{Value: "ocean_uncommon", LabelZH: "较少", LabelEN: "Less"},
		{Value: "ocean_default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "ocean_often", LabelZH: "较多", LabelEN: "More"},
		{Value: "ocean_mostly", LabelZH: "很多", LabelEN: "Lots"},
		{Value: "ocean_always", LabelZH: "大量", LabelEN: "Tons"},
		{Value: "ocean_insane", LabelZH: "疯狂", LabelEN: "Insane"},
	}

	// starting_swaps_descriptions
	startingSwapsChoices = []WorldOptionChoice{
		{Value: "classic", LabelZH: "经典", LabelEN: "Classic"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "highly random", LabelZH: "非常随机", LabelEN: "Highly Random"},
	}

	// petrification_descriptions
	petrificationChoices = []WorldOptionChoice{
		{Value: "none", LabelZH: "无", LabelEN: "None"},
		{Value: "few", LabelZH: "慢", LabelEN: "Slow"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "many", LabelZH: "快", LabelEN: "Fast"},
		{Value: "max", LabelZH: "极快", LabelEN: "Very Fast"},
	}

	// season_length_descriptions
	seasonLengthChoices = []WorldOptionChoice{
		{Value: "noseason", LabelZH: "无", LabelEN: "None"},
		{Value: "veryshortseason", LabelZH: "极短", LabelEN: "Very Short"},
		{Value: "shortseason", LabelZH: "短", LabelEN: "Short"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "longseason", LabelZH: "长", LabelEN: "Long"},
		{Value: "verylongseason", LabelZH: "极长", LabelEN: "Very Long"},
		{Value: "random", LabelZH: "随机", LabelEN: "Random"},
	}

	// size_descriptions (PC/non-console)
	worldSizeChoices = []WorldOptionChoice{
		{Value: "small", LabelZH: "小", LabelEN: "Small"},
		{Value: "medium", LabelZH: "中", LabelEN: "Medium"},
		{Value: "default", LabelZH: "大", LabelEN: "Large"},
		{Value: "huge", LabelZH: "巨大", LabelEN: "Huge"},
	}

	// day_descriptions
	dayCycleChoices = []WorldOptionChoice{
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "longday", LabelZH: "长白天", LabelEN: "Long Day"},
		{Value: "longdusk", LabelZH: "长黄昏", LabelEN: "Long Dusk"},
		{Value: "longnight", LabelZH: "长夜晚", LabelEN: "Long Night"},
		{Value: "noday", LabelZH: "无白天", LabelEN: "No Day"},
		{Value: "nodusk", LabelZH: "无黄昏", LabelEN: "No Dusk"},
		{Value: "nonight", LabelZH: "无夜晚", LabelEN: "No Night"},
		{Value: "onlyday", LabelZH: "仅白天", LabelEN: "Only Day"},
		{Value: "onlydusk", LabelZH: "仅黄昏", LabelEN: "Only Dusk"},
		{Value: "onlynight", LabelZH: "仅夜晚", LabelEN: "Only Night"},
	}

	// season_start_descriptions
	seasonStartChoices = []WorldOptionChoice{
		{Value: "default", LabelZH: "秋", LabelEN: "Autumn"},
		{Value: "winter", LabelZH: "冬", LabelEN: "Winter"},
		{Value: "spring", LabelZH: "春", LabelEN: "Spring"},
		{Value: "summer", LabelZH: "夏", LabelEN: "Summer"},
		{Value: "autumn|spring", LabelZH: "春或秋", LabelEN: "Autumn or Spring"},
		{Value: "winter|summer", LabelZH: "冬季或夏季", LabelEN: "Winter or Summer"},
		{Value: "autumn|winter|spring|summer", LabelZH: "随机", LabelEN: "Random"},
	}

	// speed_descriptions
	regrowthChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "无", LabelEN: "None"},
		{Value: "veryslow", LabelZH: "极慢", LabelEN: "Very Slow"},
		{Value: "slow", LabelZH: "慢", LabelEN: "Slow"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "fast", LabelZH: "快", LabelEN: "Fast"},
		{Value: "veryfast", LabelZH: "极快", LabelEN: "Very Fast"},
	}

	// atrium_descriptions
	atriumChoices = []WorldOptionChoice{
		{Value: "veryslow", LabelZH: "极慢", LabelEN: "Very Slow"},
		{Value: "slow", LabelZH: "慢", LabelEN: "Slow"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "fast", LabelZH: "快", LabelEN: "Fast"},
		{Value: "veryfast", LabelZH: "极快", LabelEN: "Very Fast"},
	}

	// enableddisabled_descriptions
	enabledDisabledChoices = []WorldOptionChoice{
		{Value: "none", LabelZH: "禁用", LabelEN: "Disabled"},
		{Value: "always", LabelZH: "启用", LabelEN: "Enabled"},
	}

	// yesno_descriptions
	yesNoChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "无", LabelEN: "None"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
	}

	// resetime_descriptions
	resetTimeChoices = []WorldOptionChoice{
		{Value: "none", LabelZH: "禁用", LabelEN: "Disabled"},
		{Value: "slow", LabelZH: "慢", LabelEN: "Slow"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "fast", LabelZH: "快", LabelEN: "Fast"},
		{Value: "always", LabelZH: "立刻", LabelEN: "Instant"},
	}

	// nonlethal_descriptions
	nonlethalChoices = []WorldOptionChoice{
		{Value: "nonlethal", LabelZH: "非致命", LabelEN: "Nonlethal"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
	}

	// lessdamagetaken_descriptions
	lessDamageTakenChoices = []WorldOptionChoice{
		{Value: "always", LabelZH: "较少", LabelEN: "Less"},
		{Value: "none", LabelZH: "默认", LabelEN: "Default"},
		{Value: "more", LabelZH: "较多", LabelEN: "More"},
	}

	// spawnmode_descriptions
	spawnModeChoices = []WorldOptionChoice{
		{Value: "fixed", LabelZH: "绚丽之门", LabelEN: "Florid Postern"},
		{Value: "scatter", LabelZH: "随机", LabelEN: "Random"},
	}

	// ghostenabled_descriptions
	ghostEnabledChoices = []WorldOptionChoice{
		{Value: "none", LabelZH: "更改冒险家", LabelEN: "Change Survivor"},
		{Value: "always", LabelZH: "变鬼魂", LabelEN: "Become a Ghost"},
	}

	// dropeverythingondespawn_descriptions
	dropEverythingChoices = []WorldOptionChoice{
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "always", LabelZH: "所有", LabelEN: "Everything"},
	}

	// extrastartingitems_descriptions
	extraStartingItemsChoices = []WorldOptionChoice{
		{Value: "0", LabelZH: "总是", LabelEN: "Always"},
		{Value: "5", LabelZH: "第5天后", LabelEN: "After Day 5"},
		{Value: "default", LabelZH: "第10天后", LabelEN: "After Day 10"},
		{Value: "15", LabelZH: "第15天后", LabelEN: "After Day 15"},
		{Value: "20", LabelZH: "第20天后", LabelEN: "After Day 20"},
		{Value: "none", LabelZH: "从不", LabelEN: "Never"},
	}

	// autodetect
	autoDetectChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "无", LabelEN: "None"},
		{Value: "default", LabelZH: "自动检测", LabelEN: "Auto Detect"},
		{Value: "always", LabelZH: "总是", LabelEN: "Always"},
	}

	// riftsenabled_descriptions
	riftsEnabledChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "无", LabelEN: "None"},
		{Value: "default", LabelZH: "自动检测", LabelEN: "Auto Detect"},
		{Value: "always", LabelZH: "总是", LabelEN: "Always"},
	}

	// branching_descriptions
	branchingChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "从不", LabelEN: "Never"},
		{Value: "least", LabelZH: "最少", LabelEN: "Least"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "most", LabelZH: "最多", LabelEN: "Most"},
		{Value: "random", LabelZH: "随机", LabelEN: "Random"},
	}

	// loop_descriptions
	loopChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "从不", LabelEN: "Never"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "always", LabelZH: "总是", LabelEN: "Always"},
	}

	// loop_plus_descriptions
	loopPlusChoices = []WorldOptionChoice{
		{Value: "never", LabelZH: "从不", LabelEN: "Never"},
		{Value: "rare", LabelZH: "稀有", LabelEN: "Rare"},
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "often", LabelZH: "常见", LabelEN: "Often"},
		{Value: "always", LabelZH: "总是", LabelEN: "Always"},
	}

	// specialevent_descriptions
	specialEventChoices = []WorldOptionChoice{
		{Value: "none", LabelZH: "无", LabelEN: "None"},
		{Value: "default", LabelZH: "自动", LabelEN: "Auto"},
	}

	// extraevent_descriptions
	extraEventChoices = []WorldOptionChoice{
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "enabled", LabelZH: "总是", LabelEN: "Always"},
	}

	// tasksets.GetGenTaskLists("forest")
	forestTaskSetChoices = []WorldOptionChoice{
		{Value: "default", LabelZH: "联机版", LabelEN: "Together"},
		{Value: "classic", LabelZH: "经典", LabelEN: "Classic"},
	}

	// tasksets.GetGenTaskLists("cave")
	caveTaskSetChoices = []WorldOptionChoice{
		{Value: "cave_default", LabelZH: "地下", LabelEN: "Underground"},
	}

	// startlocations.GetGenStartLocations("forest")
	forestStartLocationChoices = []WorldOptionChoice{
		{Value: "default", LabelZH: "默认", LabelEN: "Default"},
		{Value: "plus", LabelZH: "额外资源", LabelEN: "Plus"},
		{Value: "darkness", LabelZH: "黑暗", LabelEN: "Dark"},
	}

	// startlocations.GetGenStartLocations("cave")
	caveStartLocationChoices = []WorldOptionChoice{
		{Value: "caves", LabelZH: "洞穴", LabelEN: "Caves"},
	}
)

func WorldSchema(shard string) []WorldOptionGroup {
	if shard == "caves" {
		return cavesSchema
	}
	return masterSchema
}

// masterSchema mirrors the exact DST client section, group, and item ordering
// from scripts/map/customize.lua (WORLDSETTINGS_GROUP then WORLDGEN_GROUP).
// Within each group, items with an explicit client Order appear first in ascending
// Order, followed by unordered items sorted by Chinese client UTF-8 string order.
var masterSchema = []WorldOptionGroup{
	// === World Settings (世界规则) ===
	{
		ID: "settings_global", Section: "settings", NameZH: "全局", NameEN: "Global",
		Options: []WorldOptionField{
			{Key: "specialevent", LabelZH: "活动", LabelEN: "Events", Default: "default", Order: 1, Choices: specialEventChoices},
			{Key: "autumn", LabelZH: "秋", LabelEN: "Autumn", Default: "default", Order: 2, Choices: seasonLengthChoices},
			{Key: "winter", LabelZH: "冬", LabelEN: "Winter", Default: "default", Order: 3, Choices: seasonLengthChoices},
			{Key: "spring", LabelZH: "春", LabelEN: "Spring", Default: "default", Order: 4, Choices: seasonLengthChoices},
			{Key: "summer", LabelZH: "夏", LabelEN: "Summer", Default: "default", Order: 5, Choices: seasonLengthChoices},
			{Key: "day", LabelZH: "昼夜选项", LabelEN: "Day Type", Default: "default", Order: 6, Choices: dayCycleChoices},
			{Key: "spawnmode", LabelZH: "出生模式", LabelEN: "Spawn Mode", Default: "fixed", Order: 7, Choices: spawnModeChoices},
			{Key: "ghostenabled", LabelZH: "冒险家死亡", LabelEN: "Survivor Death", Default: "always", Order: 8, Choices: ghostEnabledChoices},
			{Key: "portalresurection", LabelZH: "在绚丽之门复活", LabelEN: "Revive At Florid Postern", Default: "none", Order: 9, Choices: enabledDisabledChoices},
			{Key: "ghostsanitydrain", LabelZH: "鬼魂理智值惩罚", LabelEN: "Ghost Sanity Drain", Default: "always", Order: 10, Choices: enabledDisabledChoices},
			{Key: "resettime", LabelZH: "死亡重置倒计时", LabelEN: "Death Reset Timer", Default: "default", Order: 11, Choices: resetTimeChoices},
			{Key: "beefaloheat", LabelZH: "皮弗娄牛交配频率", LabelEN: "Beefalo Mating Frequency", Default: "default", Order: 12, Choices: freqChoices},
			{Key: "krampus", LabelZH: "坎普斯", LabelEN: "Krampii", Default: "default", Order: 13, Choices: freqChoices},
		},
	},
	{
		ID: "settings_events", Section: "settings", NameZH: "活动", NameEN: "Events",
		Options: []WorldOptionField{
			{Key: "crow_carnival", LabelZH: "盛夏鸦年华", LabelEN: "Midsummer Cawnival", Default: "default", Order: 1, Choices: extraEventChoices},
			{Key: "hallowed_nights", LabelZH: "万圣夜", LabelEN: "Hallowed Nights", Default: "default", Order: 2, Choices: extraEventChoices},
			{Key: "winters_feast", LabelZH: "冬季盛宴", LabelEN: "Winter's Feast", Default: "default", Order: 3, Choices: extraEventChoices},
			{Key: "year_of_the_gobbler", LabelZH: "火鸡之年", LabelEN: "Year of the Gobbler", Default: "default", Order: 4, Choices: extraEventChoices},
			{Key: "year_of_the_varg", LabelZH: "座狼之年", LabelEN: "Year of the Varg", Default: "default", Order: 5, Choices: extraEventChoices},
			{Key: "year_of_the_pig", LabelZH: "猪王之年", LabelEN: "Year of the Pig King", Default: "default", Order: 6, Choices: extraEventChoices},
			{Key: "year_of_the_carrat", LabelZH: "胡萝卜鼠之年", LabelEN: "Year of the Carrat", Default: "default", Order: 7, Choices: extraEventChoices},
			{Key: "year_of_the_beefalo", LabelZH: "皮弗娄牛之年", LabelEN: "Year of the Beefalo", Default: "default", Order: 8, Choices: extraEventChoices},
			{Key: "year_of_the_catcoon", LabelZH: "浣猫之年", LabelEN: "Year of the Catcoon", Default: "default", Order: 9, Choices: extraEventChoices},
			{Key: "year_of_the_bunnyman", LabelZH: "兔人之年", LabelEN: "Year of the Bunnyman", Default: "default", Order: 10, Choices: extraEventChoices},
			{Key: "year_of_the_dragonfly", LabelZH: "龙蝇之年", LabelEN: "Year of the Dragonfly", Default: "default", Order: 11, Choices: extraEventChoices},
			{Key: "year_of_the_snake", LabelZH: "洞穴蠕虫之年", LabelEN: "Year of the Depths Worm", Default: "default", Order: 12, Choices: extraEventChoices},
			{Key: "year_of_the_knight", LabelZH: "发条骑士之年", LabelEN: "Year of the Clockwork Knight", Default: "default", Order: 13, Choices: extraEventChoices},
		},
	},
	{
		ID: "settings_survivors", Section: "settings", NameZH: "冒险家", NameEN: "Survivors",
		Options: []WorldOptionField{
			{Key: "extrastartingitems", LabelZH: "额外起始资源", LabelEN: "Extra Starting Resources", Default: "default", Order: 1, Choices: extraStartingItemsChoices},
			{Key: "seasonalstartingitems", LabelZH: "季节起始物品", LabelEN: "Seasonal Starting Items", Default: "default", Order: 2, Choices: yesNoChoices},
			{Key: "spawnprotection", LabelZH: "防骚扰出生保护", LabelEN: "Griefer Spawn Protection", Default: "default", Order: 3, Choices: autoDetectChoices},
			{Key: "dropeverythingondespawn", LabelZH: "离开游戏后物品掉落", LabelEN: "Drop Items on Disconnect", Default: "default", Order: 4, Choices: dropEverythingChoices},
			{Key: "healthpenalty", LabelZH: "生命值上限惩罚", LabelEN: "Max Health Penalty", Default: "always", Order: 5, Choices: enabledDisabledChoices},
			{Key: "lessdamagetaken", LabelZH: "受到的伤害", LabelEN: "Damage Taken", Default: "none", Order: 6, Choices: lessDamageTakenChoices},
			{Key: "temperaturedamage", LabelZH: "温度伤害", LabelEN: "Temperature Damage", Default: "default", Order: 7, Choices: nonlethalChoices},
			{Key: "hunger", LabelZH: "饥饿伤害", LabelEN: "Hunger Damage", Default: "default", Order: 8, Choices: nonlethalChoices},
			{Key: "darkness", LabelZH: "黑暗伤害", LabelEN: "Darkness Damage", Default: "default", Order: 9, Choices: nonlethalChoices},
			{Key: "shadowcreatures", LabelZH: "理智怪兽", LabelEN: "Sanity Monsters", Default: "default", Order: 10, Choices: freqChoices},
			{Key: "brightmarecreatures", LabelZH: "启蒙怪兽", LabelEN: "Enlightenment Monsters", Default: "default", Order: 11, Choices: freqChoices},
		},
	},
	{
		ID: "settings_misc", Section: "settings", NameZH: "世界", NameEN: "World",
		Options: []WorldOptionField{
			{Key: "hounds", LabelZH: "猎犬袭击", LabelEN: "Hound Attacks", Default: "default", Order: 1, Choices: freqChoices},
			{Key: "winterhounds", LabelZH: "寒冰猎犬群", LabelEN: "Ice Hound Waves", Default: "default", Order: 2, Choices: yesNoChoices},
			{Key: "summerhounds", LabelZH: "火焰猎犬群", LabelEN: "Fire Hound Waves", Default: "default", Order: 3, Choices: yesNoChoices},
			{Key: "lunarhail_frequency", LabelZH: "月雹", LabelEN: "Lunar Hail", Default: "default", Choices: freqChoices},
			{Key: "petrification", LabelZH: "森林石化", LabelEN: "Forest Petrification", Default: "default", Choices: petrificationChoices},
			{Key: "meteorshowers", LabelZH: "流星频率", LabelEN: "Meteor Frequency", Default: "default", Choices: freqChoices},
			{Key: "wanderingtrader_enabled", LabelZH: "流浪商人", LabelEN: "Wandering Trader", Default: "always", Choices: enabledDisabledChoices},
			{Key: "hunt", LabelZH: "狩猎", LabelEN: "Hunts", Default: "default", Choices: freqChoices},
			{Key: "alternatehunt", LabelZH: "狩猎惊喜", LabelEN: "Hunt Surprises", Default: "default", Choices: freqChoices},
			{Key: "rifts_enabled", LabelZH: "荒野裂隙", LabelEN: "Wild Rifts", Default: "default", Choices: riftsEnabledChoices},
			{Key: "rifts_frequency", LabelZH: "荒野裂隙频率", LabelEN: "Wild Rift Frequency", Default: "default", Choices: freqChoices},
			{Key: "wildfires", LabelZH: "野火", LabelEN: "Wildfires", Default: "default", Choices: freqChoices},
			{Key: "lightning", LabelZH: "闪电", LabelEN: "Lightning", Default: "default", Choices: freqChoices},
			{Key: "weather", LabelZH: "雨", LabelEN: "Rain", Default: "default", Choices: freqChoices},
			{Key: "frograin", LabelZH: "青蛙雨", LabelEN: "Frog Rain", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_resources", Section: "settings", NameZH: "资源再生", NameEN: "Resource Regrowth",
		Options: []WorldOptionField{
			{Key: "regrowth", LabelZH: "再生速度", LabelEN: "Regrowth Multiplier", Default: "default", Order: 1, Choices: regrowthChoices},
			{Key: "cactus_regrowth", LabelZH: "仙人掌", LabelEN: "Cacti", Default: "default", Choices: regrowthChoices},
			{Key: "basicresource_regrowth", LabelZH: "基础资源", LabelEN: "Basic Resources", Default: "none", Choices: enabledDisabledChoices},
			{Key: "twiggytrees_regrowth", LabelZH: "多枝树", LabelEN: "Twiggy Trees", Default: "default", Choices: regrowthChoices},
			{Key: "evergreen_regrowth", LabelZH: "常青树", LabelEN: "Evergreens", Default: "default", Choices: regrowthChoices},
			{Key: "moon_tree_regrowth", LabelZH: "月树", LabelEN: "Lune Trees", Default: "default", Choices: regrowthChoices},
			{Key: "deciduoustree_regrowth", LabelZH: "桦栗树", LabelEN: "Birchnut Trees", Default: "default", Choices: regrowthChoices},
			{Key: "palmconetree_regrowth", LabelZH: "棕榈松果树", LabelEN: "Palmcone Tree", Default: "default", Choices: regrowthChoices},
			{Key: "saltstack_regrowth", LabelZH: "盐堆", LabelEN: "Salt Formations", Default: "default", Choices: regrowthChoices},
			{Key: "carrots_regrowth", LabelZH: "胡萝卜", LabelEN: "Carrots", Default: "default", Choices: regrowthChoices},
			{Key: "reeds_regrowth", LabelZH: "芦苇", LabelEN: "Reeds", Default: "default", Choices: regrowthChoices},
			{Key: "flowers_regrowth", LabelZH: "花", LabelEN: "Flowers", Default: "default", Choices: regrowthChoices},
		},
	},
	{
		ID: "settings_portal_resources", Section: "settings", NameZH: "非自然传送门资源", NameEN: "Unnatural Portal Resources",
		Options: []WorldOptionField{
			{Key: "portal_spawnrate", LabelZH: "传送频率", LabelEN: "Portal Activity", Default: "default", Choices: freqChoices},
			{Key: "lightcrab_portalrate", LabelZH: "发光蟹", LabelEN: "Crustashines", Default: "default", Choices: freqChoices},
			{Key: "palmcone_seed_portalrate", LabelZH: "棕榈松果树芽", LabelEN: "Palmcone Sprouts", Default: "default", Choices: freqChoices},
			{Key: "powder_monkey_portalrate", LabelZH: "火药猴", LabelEN: "Powder Monkeys", Default: "default", Choices: freqChoices},
			{Key: "monkeytail_portalrate", LabelZH: "猴尾草", LabelEN: "Monkeytails", Default: "default", Choices: freqChoices},
			{Key: "bananabush_portalrate", LabelZH: "香蕉丛", LabelEN: "Banana Bushes", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_animals", Section: "settings", NameZH: "生物", NameEN: "Creatures",
		Options: []WorldOptionField{
			{Key: "gnarwail", LabelZH: "一角鲸", LabelEN: "Gnarwails", Default: "default", Choices: freqChoices},
			{Key: "penguins", LabelZH: "企鸥", LabelEN: "Pengulls", Default: "default", Choices: freqChoices},
			{Key: "bunnymen_setting", LabelZH: "兔人", LabelEN: "Bunnymen", Default: "default", Choices: freqChoices},
			{Key: "rabbits_setting", LabelZH: "兔子", LabelEN: "Rabbits", Default: "default", Choices: freqChoices},
			{Key: "otters_setting", LabelZH: "水獭掠夺者", LabelEN: "Marotters", Default: "default", Choices: freqChoices},
			{Key: "catcoons", LabelZH: "浣猫", LabelEN: "Catcoons", Default: "default", Choices: freqChoices},
			{Key: "perd", LabelZH: "火鸡", LabelEN: "Gobblers", Default: "default", Choices: freqChoices},
			{Key: "pigs_setting", LabelZH: "猪", LabelEN: "Pigs", Default: "default", Choices: freqChoices},
			{Key: "grassgekkos", LabelZH: "草壁虎转化", LabelEN: "Grass Gekko Morphing", Default: "default", Choices: freqChoices},
			{Key: "bees_setting", LabelZH: "蜜蜂", LabelEN: "Bees", Default: "default", Choices: freqChoices},
			{Key: "butterfly", LabelZH: "蝴蝶", LabelEN: "Butterflies", Default: "default", Choices: freqChoices},
			{Key: "fishschools", LabelZH: "鱼群", LabelEN: "Schools of Fish", Default: "default", Choices: freqChoices},
			{Key: "birds", LabelZH: "鸟", LabelEN: "Birds", Default: "default", Choices: freqChoices},
			{Key: "moles_setting", LabelZH: "鼹鼠", LabelEN: "Moles", Default: "default", Choices: freqChoices},
			{Key: "wobsters", LabelZH: "龙虾", LabelEN: "Wobsters", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_monsters", Section: "settings", NameZH: "敌对生物", NameEN: "Hostile Creatures",
		Options: []WorldOptionField{
			{Key: "pirateraids", LabelZH: "月亮码头海盗", LabelEN: "Moon Quay Pirates", Default: "default", Choices: freqChoices},
			{Key: "wasps", LabelZH: "杀人蜂", LabelEN: "Killer Bees", Default: "default", Choices: freqChoices},
			{Key: "walrus_setting", LabelZH: "海象", LabelEN: "MacTusk", Default: "default", Choices: freqChoices},
			{Key: "hound_mounds", LabelZH: "猎犬", LabelEN: "Hounds", Default: "default", Choices: freqChoices},
			{Key: "mosquitos", LabelZH: "蚊子", LabelEN: "Mosquitos", Default: "default", Choices: freqChoices},
			{Key: "spiders_setting", LabelZH: "蜘蛛", LabelEN: "Spiders", Default: "default", Choices: freqChoices},
			{Key: "spider_warriors", LabelZH: "蜘蛛战士", LabelEN: "Spider Warriors", Default: "default", Choices: yesNoChoices},
			{Key: "bats_setting", LabelZH: "蝙蝠", LabelEN: "Bats", Default: "default", Choices: freqChoices},
			{Key: "frogs", LabelZH: "青蛙", LabelEN: "Frogs", Default: "default", Choices: freqChoices},
			{Key: "lureplants", LabelZH: "食人花", LabelEN: "Lureplants", Default: "default", Choices: freqChoices},
			{Key: "cookiecutters", LabelZH: "饼干切割机", LabelEN: "Cookie Cutters", Default: "default", Choices: freqChoices},
			{Key: "merms", LabelZH: "鱼人", LabelEN: "Merms", Default: "default", Choices: freqChoices},
			{Key: "squid", LabelZH: "鱿鱼", LabelEN: "Skittersquids", Default: "default", Choices: freqChoices},
			{Key: "sharks", LabelZH: "鲨鱼", LabelEN: "Sharks", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_giants", Section: "settings", NameZH: "巨兽", NameEN: "Giants",
		Options: []WorldOptionField{
			{Key: "klaus", LabelZH: "克劳斯", LabelEN: "Klaus", Default: "default", Choices: freqChoices},
			{Key: "sharkboi", LabelZH: "大霜鲨", LabelEN: "Frostjaw", Default: "default", Choices: freqChoices},
			{Key: "crabking", LabelZH: "帝王蟹", LabelEN: "Crabking", Default: "default", Choices: freqChoices},
			{Key: "eyeofterror", LabelZH: "恐怖之眼", LabelEN: "Eye of Terror", Default: "default", Choices: freqChoices},
			{Key: "daywalker2", LabelZH: "拾荒疯猪", LabelEN: "Scrappy Werepig", Default: "default", Choices: freqChoices},
			{Key: "fruitfly", LabelZH: "果蝇王", LabelEN: "Lord of the Fruit Flies", Default: "default", Choices: freqChoices},
			{Key: "liefs", LabelZH: "树精守卫", LabelEN: "Treeguards", Default: "default", Choices: freqChoices},
			{Key: "deciduousmonster", LabelZH: "毒桦栗树", LabelEN: "Poison Birchnut Trees", Default: "default", Choices: freqChoices},
			{Key: "bearger", LabelZH: "熊獾", LabelEN: "Bearger", Default: "default", Choices: freqChoices},
			{Key: "deerclops", LabelZH: "独眼巨鹿", LabelEN: "Deerclops", Default: "default", Choices: freqChoices},
			{Key: "antliontribute", LabelZH: "蚁狮贡品", LabelEN: "Antlion Tribute", Default: "default", Choices: freqChoices},
			{Key: "beequeen", LabelZH: "蜂王", LabelEN: "Bee Queen", Default: "default", Choices: freqChoices},
			{Key: "spiderqueen", LabelZH: "蜘蛛女王", LabelEN: "Spider Queen", Default: "default", Choices: freqChoices},
			{Key: "malbatross", LabelZH: "邪天翁", LabelEN: "Malbatross", Default: "default", Choices: freqChoices},
			{Key: "goosemoose", LabelZH: "麋鹿鹅", LabelEN: "Meese/Geese", Default: "default", Choices: freqChoices},
			{Key: "dragonfly", LabelZH: "龙蝇", LabelEN: "Dragonfly", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_lunar_mutations", Section: "settings", NameZH: "月亮变异", NameEN: "Lunar Mutations",
		Options: []WorldOptionField{
			{Key: "mutated_bird_gestalt", LabelZH: "亮喙鸟", LabelEN: "Bright-Beaked Birds", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_birds", LabelZH: "变异的鸟", LabelEN: "Mutated Birds", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_merm", LabelZH: "变异鱼人", LabelEN: "Mutated Merms", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_hounds", LabelZH: "恐怖猎犬", LabelEN: "Horror Hounds", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_deerclops", LabelZH: "晶体独眼巨鹿", LabelEN: "Crystal Deerclops", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_buzzard_gestalt", LabelZH: "水晶冠秃鹫", LabelEN: "Crystal-Crested Buzzards", Default: "default", Choices: yesNoChoices},
			{Key: "penguins_moon", LabelZH: "永冻企鸥", LabelEN: "Permafrost Pengulls", Default: "default", Choices: yesNoChoices},
			{Key: "moon_spider", LabelZH: "破碎蜘蛛", LabelEN: "Shattered Spiders", Default: "default", Choices: freqChoices},
			{Key: "mutated_spiderqueen", LabelZH: "破碎蜘蛛洞", LabelEN: "Shattered Spider Holes", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_bearger", LabelZH: "装甲熊獾", LabelEN: "Armored Bearger", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_warg", LabelZH: "附身座狼", LabelEN: "Possessed Vargs", Default: "default", Choices: yesNoChoices},
		},
	},
	// === World Generation (世界生成) ===
	{
		ID: "worldgen_global", Section: "worldgen", NameZH: "全局", NameEN: "Global",
		Options: []WorldOptionField{
			{Key: "season_start", LabelZH: "起始季节", LabelEN: "Starting Season", Default: "default", Order: 1, Choices: seasonStartChoices},
		},
	},
	{
		ID: "worldgen_misc", Section: "worldgen", NameZH: "世界", NameEN: "World",
		Options: []WorldOptionField{
			{Key: "task_set", LabelZH: "生物群落", LabelEN: "Biomes", Default: "default", Order: 1, Choices: forestTaskSetChoices},
			{Key: "start_location", LabelZH: "出生点", LabelEN: "Spawn Area", Default: "default", Order: 2, Choices: forestStartLocationChoices},
			{Key: "world_size", LabelZH: "世界大小", LabelEN: "World Size", Default: "default", Order: 3, Choices: worldSizeChoices},
			{Key: "branching", LabelZH: "分支", LabelEN: "Branches", Default: "default", Order: 4, Choices: branchingChoices},
			{Key: "loop", LabelZH: "环形", LabelEN: "Loops", Default: "default", Order: 5, Choices: loopChoices},
			{Key: "roads", LabelZH: "道路", LabelEN: "Roads", Default: "default", Order: 6, Choices: yesNoChoices},
			{Key: "touchstone", LabelZH: "试金石", LabelEN: "Touch Stones", Default: "default", Order: 17, Choices: worldgenFreqChoices},
			{Key: "boons", LabelZH: "失败的冒险家", LabelEN: "Failed Survivors", Default: "default", Order: 18, Choices: worldgenFreqChoices},
			{Key: "prefabswaps_start", LabelZH: "开始资源多样化", LabelEN: "Starting Resource Variety", Default: "default", Order: 20, Choices: startingSwapsChoices},
			{Key: "junkyard", LabelZH: "垃圾场", LabelEN: "Junk Yard", Default: "default", Choices: yesNoChoices},
			{Key: "moon_fissure", LabelZH: "天体裂缝", LabelEN: "Celestial Fissures", Default: "default", Choices: worldgenFreqChoices},
			{Key: "balatro", LabelZH: "小丑", LabelEN: "JIMBO", Default: "default", Choices: yesNoChoices},
			{Key: "terrariumchest", LabelZH: "盒中泰拉", LabelEN: "Terrarium", Default: "default", Choices: yesNoChoices},
			{Key: "stageplays", LabelZH: "舞台剧", LabelEN: "Stage Plays", Default: "default", Choices: yesNoChoices},
		},
	},
	{
		ID: "worldgen_resources", Section: "worldgen", NameZH: "资源", NameEN: "Resources",
		Options: []WorldOptionField{
			{Key: "cactus", LabelZH: "仙人掌", LabelEN: "Cacti", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ocean_bullkelp", LabelZH: "公牛海带", LabelEN: "Bull Kelp", Default: "default", Choices: worldgenFreqChoices},
			{Key: "marshbush", LabelZH: "尖刺灌木", LabelEN: "Spiky Bushes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rock", LabelZH: "巨石", LabelEN: "Boulders", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_sapling", LabelZH: "月亮树苗", LabelEN: "Lunar Saplings", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_rock", LabelZH: "月亮石", LabelEN: "Lunar Rocks", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_tree", LabelZH: "月树", LabelEN: "Lune Trees", Default: "default", Choices: worldgenFreqChoices},
			{Key: "sapling", LabelZH: "树苗", LabelEN: "Saplings", Default: "default", Choices: worldgenFreqChoices},
			{Key: "trees", LabelZH: "树（所有）", LabelEN: "Trees (All)", Default: "default", Choices: worldgenFreqChoices},
			{Key: "palmconetree", LabelZH: "棕榈松果树", LabelEN: "Palmcone Tree", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ponds", LabelZH: "池塘", LabelEN: "Ponds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "meteorspawner", LabelZH: "流星区域", LabelEN: "Meteor Fields", Default: "default", Choices: worldgenFreqChoices},
			{Key: "berrybush", LabelZH: "浆果丛", LabelEN: "Berry Bushes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_bullkelp", LabelZH: "海岸公牛海带", LabelEN: "Beached Bull Kelp", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_starfish", LabelZH: "海星", LabelEN: "Anenemies", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ocean_seastack", LabelZH: "海蚀柱", LabelEN: "Sea Stacks", Default: "ocean_default", Choices: oceanWorldgenFreqChoices},
			{Key: "moon_hotspring", LabelZH: "温泉", LabelEN: "Hot Springs", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flint", LabelZH: "燧石", LabelEN: "Flint", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_berrybush", LabelZH: "石果灌木丛", LabelEN: "Stone Fruit Bushes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "carrot", LabelZH: "胡萝卜", LabelEN: "Carrots", Default: "default", Choices: worldgenFreqChoices},
			{Key: "reeds", LabelZH: "芦苇", LabelEN: "Reeds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flowers", LabelZH: "花，邪恶花", LabelEN: "Flowers, Evil Flowers", Default: "default", Choices: worldgenFreqChoices},
			{Key: "grass", LabelZH: "草", LabelEN: "Grass", Default: "default", Choices: worldgenFreqChoices},
			{Key: "mushroom", LabelZH: "蘑菇", LabelEN: "Mushrooms", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rock_ice", LabelZH: "迷你冰川", LabelEN: "Mini Glaciers", Default: "default", Choices: worldgenFreqChoices},
			{Key: "tumbleweed", LabelZH: "风滚草", LabelEN: "Tumbleweeds", Default: "default", Choices: worldgenFreqChoices},
		},
	},
	{
		ID: "worldgen_animals", Section: "worldgen", NameZH: "生物以及刷新点", NameEN: "Creatures and Spawners",
		Options: []WorldOptionField{
			{Key: "lightninggoat", LabelZH: "伏特羊", LabelEN: "Volt Goats", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rabbits", LabelZH: "兔洞", LabelEN: "Rabbit Holes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ocean_otterdens", LabelZH: "水獭掠夺者窝点", LabelEN: "Marotter Dens", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_fruitdragon", LabelZH: "沙拉蝾螈", LabelEN: "Saladmander", Default: "default", Choices: worldgenFreqChoices},
			{Key: "pigs", LabelZH: "猪屋", LabelEN: "Pig Houses", Default: "default", Choices: worldgenFreqChoices},
			{Key: "beefalo", LabelZH: "皮弗娄牛", LabelEN: "Beefalos", Default: "default", Choices: worldgenFreqChoices},
			{Key: "buzzard", LabelZH: "秃鹫", LabelEN: "Buzzards", Default: "default", Choices: worldgenFreqChoices},
			{Key: "catcoon", LabelZH: "空心树桩", LabelEN: "Hollow Stump", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_carrot", LabelZH: "胡萝卜鼠", LabelEN: "Carrats", Default: "default", Choices: worldgenFreqChoices},
			{Key: "bees", LabelZH: "蜜蜂蜂窝", LabelEN: "Bee Hives", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ocean_shoal", LabelZH: "鱼群", LabelEN: "Shoals", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moles", LabelZH: "鼹鼠丘", LabelEN: "Mole Burrows", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ocean_wobsterden", LabelZH: "龙虾窝", LabelEN: "Wobster Mounds", Default: "default", Choices: worldgenFreqChoices},
		},
	},
	{
		ID: "worldgen_monsters", Section: "worldgen", NameZH: "敌对生物以及刷新点", NameEN: "Hostile Creatures and Spawners",
		Options: []WorldOptionField{
			{Key: "chess", LabelZH: "发条装置", LabelEN: "Clockworks", Default: "default", Choices: worldgenFreqChoices},
			{Key: "angrybees", LabelZH: "杀人蜂蜂窝", LabelEN: "Killer Bee Hives", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ocean_waterplant", LabelZH: "海草", LabelEN: "Sea Weeds", Default: "ocean_default", Choices: oceanWorldgenFreqChoices},
			{Key: "walrus", LabelZH: "海象营地", LabelEN: "MacTusk Camps", Default: "default", Choices: worldgenFreqChoices},
			{Key: "merm", LabelZH: "漏雨的小屋", LabelEN: "Leaky Shack", Default: "default", Choices: worldgenFreqChoices},
			{Key: "houndmound", LabelZH: "猎犬丘", LabelEN: "Hound Mounds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "moon_spiders", LabelZH: "破碎蜘蛛洞", LabelEN: "Shattered Spider Holes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "spiders", LabelZH: "蜘蛛巢", LabelEN: "Spider Dens", Default: "default", Choices: worldgenFreqChoices},
			{Key: "tentacles", LabelZH: "触手", LabelEN: "Tentacles", Default: "default", Choices: worldgenFreqChoices},
			{Key: "tallbirds", LabelZH: "高脚鸟", LabelEN: "Tallbirds", Default: "default", Choices: worldgenFreqChoices},
		},
	},
}

// cavesSchema mirrors the exact DST client section, group, and item ordering
// for cave sub-worlds (location="cave", is_master_world=false).
var cavesSchema = []WorldOptionGroup{
	// === World Settings (世界规则) ===
	{
		ID: "settings_misc", Section: "settings", NameZH: "世界", NameEN: "World",
		Options: []WorldOptionField{
			{Key: "earthquakes", LabelZH: "地震", LabelEN: "Earthquakes", Default: "default", Choices: freqChoices},
			{Key: "wormattacks_boss", LabelZH: "大蠕虫", LabelEN: "Great Worm", Default: "default", Choices: loopPlusChoices},
			{Key: "wormattacks", LabelZH: "洞穴蠕虫袭击", LabelEN: "Cave Worm Attacks", Default: "default", Choices: freqChoices},
			{Key: "rifts_enabled_cave", LabelZH: "荒野裂隙", LabelEN: "Wild Rifts", Default: "default", Choices: riftsEnabledChoices},
			{Key: "rifts_frequency_cave", LabelZH: "荒野裂隙频率", LabelEN: "Wild Rift Frequency", Default: "default", Choices: freqChoices},
			{Key: "atriumgate", LabelZH: "远古大门", LabelEN: "Ancient Gateway", Default: "default", Choices: atriumChoices},
			{Key: "acidrain_enabled", LabelZH: "酸雨", LabelEN: "Acid Rain", Default: "always", Choices: enabledDisabledChoices},
			{Key: "weather", LabelZH: "雨", LabelEN: "Rain", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_resources", Section: "settings", NameZH: "资源再生", NameEN: "Resource Regrowth",
		Options: []WorldOptionField{
			{Key: "regrowth", LabelZH: "再生速度", LabelEN: "Regrowth Multiplier", Default: "default", Order: 1, Choices: regrowthChoices},
			{Key: "lightflier_flower_regrowth", LabelZH: "光虫花", LabelEN: "Lightbug Flower", Default: "default", Choices: regrowthChoices},
			{Key: "twiggytrees_regrowth", LabelZH: "多枝树", LabelEN: "Twiggy Trees", Default: "default", Choices: regrowthChoices},
			{Key: "tree_rock_regrowth", LabelZH: "巨石枝", LabelEN: "Boulderboughs", Default: "default", Choices: regrowthChoices},
			{Key: "evergreen_regrowth", LabelZH: "常青树", LabelEN: "Evergreens", Default: "default", Choices: regrowthChoices},
			{Key: "mushtree_moon_regrowth", LabelZH: "月亮蘑菇树", LabelEN: "Lunar Mushtrees", Default: "default", Choices: regrowthChoices},
			{Key: "reeds_regrowth", LabelZH: "芦苇", LabelEN: "Reeds", Default: "default", Choices: regrowthChoices},
			{Key: "flower_cave_regrowth", LabelZH: "荧光花", LabelEN: "Light Flower", Default: "default", Choices: regrowthChoices},
			{Key: "mushtree_regrowth", LabelZH: "蘑菇树", LabelEN: "Mushroom Trees", Default: "default", Choices: regrowthChoices},
		},
	},
	{
		ID: "settings_animals", Section: "settings", NameZH: "生物", NameEN: "Creatures",
		Options: []WorldOptionField{
			{Key: "bunnymen_setting", LabelZH: "兔人", LabelEN: "Bunnymen", Default: "default", Choices: freqChoices},
			{Key: "dustmoths", LabelZH: "尘蛾", LabelEN: "Dust Moths", Default: "default", Choices: freqChoices},
			{Key: "pigs_setting", LabelZH: "猪", LabelEN: "Pigs", Default: "default", Choices: freqChoices},
			{Key: "lightfliers", LabelZH: "球状光虫", LabelEN: "Bulbous Lightbugs", Default: "default", Choices: freqChoices},
			{Key: "rocky_setting", LabelZH: "石虾", LabelEN: "Rock Lobsters", Default: "default", Choices: freqChoices},
			{Key: "monkey_setting", LabelZH: "穴居猴", LabelEN: "Splumonkeys", Default: "default", Choices: freqChoices},
			{Key: "grassgekkos", LabelZH: "草壁虎转化", LabelEN: "Grass Gekko Morphing", Default: "default", Choices: freqChoices},
			{Key: "mushgnome", LabelZH: "蘑菇地精", LabelEN: "Mush Gnomes", Default: "default", Choices: freqChoices},
			{Key: "slurtles_setting", LabelZH: "蛞蝓龟", LabelEN: "Slurtles", Default: "default", Choices: freqChoices},
			{Key: "snurtles", LabelZH: "蜗牛龟", LabelEN: "Snurtles", Default: "default", Choices: freqChoices},
			{Key: "moles_setting", LabelZH: "鼹鼠", LabelEN: "Moles", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_monsters", Section: "settings", NameZH: "敌对生物", NameEN: "Hostile Creatures",
		Options: []WorldOptionField{
			{Key: "spider_spitter", LabelZH: "喷射蜘蛛", LabelEN: "Spitter Spiders", Default: "default", Choices: freqChoices},
			{Key: "rocky_boss", LabelZH: "岩石大师", LabelEN: "Rock Masters", Default: "default", Choices: freqChoices},
			{Key: "itemmimics", LabelZH: "拟态蠕虫", LabelEN: "Mimicreeps", Default: "default", Choices: freqChoices},
			{Key: "chest_mimics", LabelZH: "暴躁箱子", LabelEN: "Ornery Chests", Default: "default", Choices: freqChoices},
			{Key: "spider_hider", LabelZH: "洞穴蜘蛛", LabelEN: "Cave Spiders", Default: "default", Choices: freqChoices},
			{Key: "bat_boss", LabelZH: "猎犬蝙蝠", LabelEN: "Hound Batilisks", Default: "default", Choices: freqChoices},
			{Key: "spider_dropper", LabelZH: "穴居悬蛛", LabelEN: "Dangling Depth Dwellers", Default: "default", Choices: freqChoices},
			{Key: "spiders_setting", LabelZH: "蜘蛛", LabelEN: "Spiders", Default: "default", Choices: freqChoices},
			{Key: "spider_warriors", LabelZH: "蜘蛛战士", LabelEN: "Spider Warriors", Default: "default", Choices: yesNoChoices},
			{Key: "bats_setting", LabelZH: "蝙蝠", LabelEN: "Bats", Default: "default", Choices: freqChoices},
			{Key: "molebats", LabelZH: "裸鼹蝠", LabelEN: "Naked Mole Bats", Default: "default", Choices: freqChoices},
			{Key: "nightmarecreatures", LabelZH: "遗迹梦魇", LabelEN: "Ruins Nightmares", Default: "default", Choices: freqChoices},
			{Key: "merms", LabelZH: "鱼人", LabelEN: "Merms", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_giants", Section: "settings", NameZH: "巨兽", NameEN: "Giants",
		Options: []WorldOptionField{
			{Key: "fruitfly", LabelZH: "果蝇王", LabelEN: "Lord of the Fruit Flies", Default: "default", Choices: freqChoices},
			{Key: "liefs", LabelZH: "树精守卫", LabelEN: "Treeguards", Default: "default", Choices: freqChoices},
			{Key: "daywalker", LabelZH: "梦魇疯猪", LabelEN: "Nightmare Werepig", Default: "default", Choices: freqChoices},
			{Key: "toadstool", LabelZH: "毒菌蟾蜍", LabelEN: "Toadstool", Default: "default", Choices: freqChoices},
			{Key: "spiderqueen", LabelZH: "蜘蛛女王", LabelEN: "Spider Queen", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_lunar_mutations", Section: "settings", NameZH: "月亮变异", NameEN: "Lunar Mutations",
		Options: []WorldOptionField{
			{Key: "mutated_birds", LabelZH: "变异的鸟", LabelEN: "Mutated Birds", Default: "default", Choices: yesNoChoices},
			{Key: "mutated_merm", LabelZH: "变异鱼人", LabelEN: "Mutated Merms", Default: "default", Choices: yesNoChoices},
			{Key: "moon_spider", LabelZH: "破碎蜘蛛", LabelEN: "Shattered Spiders", Default: "default", Choices: freqChoices},
			{Key: "mutated_spiderqueen", LabelZH: "破碎蜘蛛洞", LabelEN: "Shattered Spider Holes", Default: "default", Choices: yesNoChoices},
		},
	},
	// === World Generation (世界生成) ===
	{
		ID: "worldgen_misc", Section: "worldgen", NameZH: "世界", NameEN: "World",
		Options: []WorldOptionField{
			{Key: "task_set", LabelZH: "生物群落", LabelEN: "Biomes", Default: "cave_default", Order: 1, Choices: caveTaskSetChoices},
			{Key: "start_location", LabelZH: "出生点", LabelEN: "Spawn Area", Default: "caves", Order: 2, Choices: caveStartLocationChoices},
			{Key: "world_size", LabelZH: "世界大小", LabelEN: "World Size", Default: "default", Order: 3, Choices: worldSizeChoices},
			{Key: "branching", LabelZH: "分支", LabelEN: "Branches", Default: "default", Order: 4, Choices: branchingChoices},
			{Key: "loop", LabelZH: "环形", LabelEN: "Loops", Default: "default", Order: 5, Choices: loopChoices},
			{Key: "touchstone", LabelZH: "试金石", LabelEN: "Touch Stones", Default: "default", Order: 17, Choices: worldgenFreqChoices},
			{Key: "boons", LabelZH: "失败的冒险家", LabelEN: "Failed Survivors", Default: "default", Order: 18, Choices: worldgenFreqChoices},
			{Key: "cavelight", LabelZH: "洞穴光照", LabelEN: "Sinkhole Lights", Default: "default", Order: 18, Choices: regrowthChoices},
			{Key: "prefabswaps_start", LabelZH: "开始资源多样化", LabelEN: "Starting Resource Variety", Default: "default", Order: 20, Choices: startingSwapsChoices},
		},
	},
	{
		ID: "worldgen_resources", Section: "worldgen", NameZH: "资源", NameEN: "Resources",
		Options: []WorldOptionField{
			{Key: "wormlights", LabelZH: "发光浆果", LabelEN: "Glow Berries", Default: "default", Choices: worldgenFreqChoices},
			{Key: "marshbush", LabelZH: "尖刺灌木", LabelEN: "Spiky Bushes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rock", LabelZH: "巨石", LabelEN: "Boulders", Default: "default", Choices: worldgenFreqChoices},
			{Key: "tree_rock", LabelZH: "巨石枝", LabelEN: "Boulderboughs", Default: "default", Choices: worldgenFreqChoices},
			{Key: "sapling", LabelZH: "树苗", LabelEN: "Saplings", Default: "default", Choices: worldgenFreqChoices},
			{Key: "trees", LabelZH: "树（所有）", LabelEN: "Trees (All)", Default: "default", Choices: worldgenFreqChoices},
			{Key: "cave_ponds", LabelZH: "池塘", LabelEN: "Ponds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "fern", LabelZH: "洞穴蕨类", LabelEN: "Cave Ferns", Default: "default", Choices: worldgenFreqChoices},
			{Key: "berrybush", LabelZH: "浆果丛", LabelEN: "Berry Bushes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flint", LabelZH: "燧石", LabelEN: "Flint", Default: "default", Choices: worldgenFreqChoices},
			{Key: "reeds", LabelZH: "芦苇", LabelEN: "Reeds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "lichen", LabelZH: "苔藓", LabelEN: "Lichen", Default: "default", Choices: worldgenFreqChoices},
			{Key: "grass", LabelZH: "草", LabelEN: "Grass", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flower_cave", LabelZH: "荧光花", LabelEN: "Light Flowers", Default: "default", Choices: worldgenFreqChoices},
			{Key: "mushroom", LabelZH: "蘑菇", LabelEN: "Mushrooms", Default: "default", Choices: worldgenFreqChoices},
			{Key: "mushtree", LabelZH: "蘑菇树", LabelEN: "Mushroom Trees", Default: "default", Choices: worldgenFreqChoices},
			{Key: "banana", LabelZH: "香蕉", LabelEN: "Bananas", Default: "default", Choices: worldgenFreqChoices},
		},
	},
	{
		ID: "worldgen_animals", Section: "worldgen", NameZH: "生物以及刷新点", NameEN: "Creatures and Spawners",
		Options: []WorldOptionField{
			{Key: "bunnymen", LabelZH: "兔屋", LabelEN: "Rabbit Hutches", Default: "default", Choices: worldgenFreqChoices},
			{Key: "slurper", LabelZH: "啜食者", LabelEN: "Slurpers", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rocky", LabelZH: "石虾", LabelEN: "Rock Lobsters", Default: "default", Choices: worldgenFreqChoices},
			{Key: "monkey", LabelZH: "穴居猴桶", LabelEN: "Splumonkey Pods", Default: "default", Choices: worldgenFreqChoices},
			{Key: "slurtles", LabelZH: "蛞蝓龟窝", LabelEN: "Slurtle Mounds", Default: "default", Choices: worldgenFreqChoices},
		},
	},
	{
		ID: "worldgen_monsters", Section: "worldgen", NameZH: "敌对生物以及刷新点", NameEN: "Hostile Creatures and Spawners",
		Options: []WorldOptionField{
			{Key: "chess", LabelZH: "发条装置", LabelEN: "Clockworks", Default: "default", Choices: worldgenFreqChoices},
			{Key: "fissure", LabelZH: "梦魇裂缝", LabelEN: "Nightmare Fissures", Default: "default", Choices: worldgenFreqChoices},
			{Key: "worms", LabelZH: "洞穴蠕虫", LabelEN: "Cave Worms", Default: "default", Choices: worldgenFreqChoices},
			{Key: "cave_spiders", LabelZH: "蛛网岩", LabelEN: "Spilagmites", Default: "default", Choices: worldgenFreqChoices},
			{Key: "spiders", LabelZH: "蜘蛛巢", LabelEN: "Spider Dens", Default: "default", Choices: worldgenFreqChoices},
			{Key: "bats", LabelZH: "蝙蝠", LabelEN: "Bats", Default: "default", Choices: worldgenFreqChoices},
			{Key: "tentacles", LabelZH: "触手", LabelEN: "Tentacles", Default: "default", Choices: worldgenFreqChoices},
		},
	},
}
