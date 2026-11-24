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
		{Value: "default", LabelZH: "大 (默认)", LabelEN: "Large (Default)"},
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

	// enableddisabled_descriptions
	enabledDisabledChoices = []WorldOptionChoice{
		{Value: "none", LabelZH: "禁用", LabelEN: "Disabled"},
		{Value: "always", LabelZH: "启用", LabelEN: "Enabled"},
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

	// specialevent: none/default + official SPECIAL_EVENTS values supported by ApplySpecialEvent()
	specialEventChoices = []WorldOptionChoice{
		{Value: "none", LabelZH: "无", LabelEN: "None"},
		{Value: "default", LabelZH: "自动", LabelEN: "Auto"},
		{Value: "crow_carnival", LabelZH: "盛夏鸦年华", LabelEN: "Midsummer Cawnival"},
		{Value: "hallowed_nights", LabelZH: "万圣夜", LabelEN: "Hallowed Nights"},
		{Value: "winters_feast", LabelZH: "冬季盛宴", LabelEN: "Winter's Feast"},
		{Value: "year_of_the_gobbler", LabelZH: "火鸡之年", LabelEN: "Year of the Gobbler"},
		{Value: "year_of_the_varg", LabelZH: "座狼之年", LabelEN: "Year of the Varg"},
		{Value: "year_of_the_pig", LabelZH: "猪王之年", LabelEN: "Year of the Pig King"},
		{Value: "year_of_the_carrat", LabelZH: "胡萝卜鼠之年", LabelEN: "Year of the Carrat"},
		{Value: "year_of_the_beefalo", LabelZH: "皮弗娄牛之年", LabelEN: "Year of the Beefalo"},
		{Value: "year_of_the_catcoon", LabelZH: "浣猫之年", LabelEN: "Year of the Catcoon"},
		{Value: "year_of_the_bunnyman", LabelZH: "兔人之年", LabelEN: "Year of the Bunnyman"},
		{Value: "year_of_the_dragonfly", LabelZH: "龙蝇之年", LabelEN: "Year of the Dragonfly"},
	}
)

func WorldSchema(shard string) []WorldOptionGroup {
	if shard == "caves" {
		return cavesSchema
	}
	return masterSchema
}

// masterSchema follows the exact DST client section, group, and item ordering
// from scripts/map/customize.lua (WORLDSETTINGS_GROUP then WORLDGEN_GROUP).
// Within each group, items with an explicit client Order appear first in ascending
// Order, followed by unordered items sorted by the Chinese client UTF-8 string order.
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
			{Key: "portalresurection", LabelZH: "在绚丽之门复活", LabelEN: "Revive At Florid Postern", Default: "none", Order: 9, Choices: enabledDisabledChoices},
			{Key: "ghostsanitydrain", LabelZH: "鬼魂理智值惩罚", LabelEN: "Ghost Sanity Drain", Default: "always", Order: 10, Choices: enabledDisabledChoices},
			{Key: "resettime", LabelZH: "死亡重置倒计时", LabelEN: "Death Reset Timer", Default: "default", Order: 11, Choices: resetTimeChoices},
			{Key: "beefaloheat", LabelZH: "皮弗娄牛交配频率", LabelEN: "Beefalo Mating Frequency", Default: "default", Order: 12, Choices: freqChoices},
			{Key: "krampus", LabelZH: "坎普斯", LabelEN: "Krampii", Default: "default", Order: 13, Choices: freqChoices},
		},
	},
	{
		ID: "settings_survivors", Section: "settings", NameZH: "冒险家", NameEN: "Survivors",
		Options: []WorldOptionField{
			{Key: "healthpenalty", LabelZH: "血量上限惩罚", LabelEN: "Max Health Penalty", Default: "always", Order: 5, Choices: enabledDisabledChoices},
			{Key: "temperaturedamage", LabelZH: "温度伤害", LabelEN: "Temperature Damage", Default: "default", Order: 7, Choices: nonlethalChoices},
			{Key: "hunger", LabelZH: "饥饿伤害", LabelEN: "Hunger Damage", Default: "default", Order: 8, Choices: nonlethalChoices},
			{Key: "darkness", LabelZH: "黑暗伤害", LabelEN: "Darkness Damage", Default: "default", Order: 9, Choices: nonlethalChoices},
		},
	},
	{
		ID: "settings_misc", Section: "settings", NameZH: "世界", NameEN: "World",
		Options: []WorldOptionField{
			{Key: "hounds", LabelZH: "猎犬袭击", LabelEN: "Hound Attacks", Default: "default", Order: 1, Choices: freqChoices},
			{Key: "petrification", LabelZH: "森林石化", LabelEN: "Forest Petrification", Default: "default", Choices: petrificationChoices},
			{Key: "hunt", LabelZH: "狩猎", LabelEN: "Hunts", Default: "default", Choices: freqChoices},
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
			{Key: "basicresource_regrowth", LabelZH: "基础资源", LabelEN: "Basic Resources", Default: "none", Choices: enabledDisabledChoices},
		},
	},
	{
		ID: "settings_giants", Section: "settings", NameZH: "巨兽", NameEN: "Giants",
		Options: []WorldOptionField{
			{Key: "klaus", LabelZH: "克劳斯", LabelEN: "Klaus", Default: "default", Choices: freqChoices},
			{Key: "crabking", LabelZH: "帝王蟹", LabelEN: "Crabking", Default: "default", Choices: freqChoices},
			{Key: "bearger", LabelZH: "熊獾", LabelEN: "Bearger", Default: "default", Choices: freqChoices},
			{Key: "deerclops", LabelZH: "独眼巨鹿", LabelEN: "Deerclops", Default: "default", Choices: freqChoices},
			{Key: "antliontribute", LabelZH: "蚁狮贡品", LabelEN: "Antlion Tribute", Default: "default", Choices: freqChoices},
			{Key: "beequeen", LabelZH: "蜂王", LabelEN: "Bee Queen", Default: "default", Choices: freqChoices},
			{Key: "malbatross", LabelZH: "邪天翁", LabelEN: "Malbatross", Default: "default", Choices: freqChoices},
			{Key: "goosemoose", LabelZH: "麋鹿鹅", LabelEN: "Meese/Geese", Default: "default", Choices: freqChoices},
			{Key: "dragonfly", LabelZH: "龙蝇", LabelEN: "Dragonfly", Default: "default", Choices: freqChoices},
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
			{Key: "world_size", LabelZH: "世界大小", LabelEN: "World Size", Default: "default", Order: 3, Choices: worldSizeChoices},
			{Key: "branching", LabelZH: "分支", LabelEN: "Branches", Default: "default", Order: 4, Choices: branchingChoices},
			{Key: "loop", LabelZH: "环形", LabelEN: "Loops", Default: "default", Order: 5, Choices: loopChoices},
		},
	},
	{
		ID: "worldgen_resources", Section: "worldgen", NameZH: "资源", NameEN: "Resources",
		Options: []WorldOptionField{
			{Key: "cactus", LabelZH: "仙人掌", LabelEN: "Cacti", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rock", LabelZH: "巨石", LabelEN: "Boulders", Default: "default", Choices: worldgenFreqChoices},
			{Key: "sapling", LabelZH: "树苗", LabelEN: "Saplings", Default: "default", Choices: worldgenFreqChoices},
			{Key: "trees", LabelZH: "树（所有）", LabelEN: "Trees (All)", Default: "default", Choices: worldgenFreqChoices},
			{Key: "ponds", LabelZH: "池塘", LabelEN: "Ponds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "meteorspawner", LabelZH: "流星区域", LabelEN: "Meteor Fields", Default: "default", Choices: worldgenFreqChoices},
			{Key: "berrybush", LabelZH: "浆果丛", LabelEN: "Berry Bushes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flint", LabelZH: "燧石", LabelEN: "Flint", Default: "default", Choices: worldgenFreqChoices},
			{Key: "carrot", LabelZH: "胡萝卜", LabelEN: "Carrots", Default: "default", Choices: worldgenFreqChoices},
			{Key: "reeds", LabelZH: "芦苇", LabelEN: "Reeds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flowers", LabelZH: "花，邪恶花", LabelEN: "Flowers, Evil Flowers", Default: "default", Choices: worldgenFreqChoices},
			{Key: "grass", LabelZH: "草", LabelEN: "Grass", Default: "default", Choices: worldgenFreqChoices},
			{Key: "mushroom", LabelZH: "蘑菇", LabelEN: "Mushrooms", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rock_ice", LabelZH: "迷你冰川", LabelEN: "Mini Glaciers", Default: "default", Choices: worldgenFreqChoices},
		},
	},
	{
		ID: "worldgen_animals", Section: "worldgen", NameZH: "生物以及刷新点", NameEN: "Creatures and Spawners",
		Options: []WorldOptionField{
			{Key: "lightninggoat", LabelZH: "伏特羊", LabelEN: "Volt Goats", Default: "default", Choices: worldgenFreqChoices},
			{Key: "rabbits", LabelZH: "兔洞", LabelEN: "Rabbit Holes", Default: "default", Choices: worldgenFreqChoices},
			{Key: "pigs", LabelZH: "猪屋", LabelEN: "Pig Houses", Default: "default", Choices: worldgenFreqChoices},
			{Key: "beefalo", LabelZH: "皮弗娄牛", LabelEN: "Beefalos", Default: "default", Choices: worldgenFreqChoices},
			{Key: "catcoon", LabelZH: "空心树桩", LabelEN: "Hollow Stump", Default: "default", Choices: worldgenFreqChoices},
			{Key: "bees", LabelZH: "蜜蜂蜂窝", LabelEN: "Bee Hives", Default: "default", Choices: worldgenFreqChoices},
		},
	},
	{
		ID: "worldgen_monsters", Section: "worldgen", NameZH: "敌对生物以及刷新点", NameEN: "Hostile Creatures and Spawners",
		Options: []WorldOptionField{
			{Key: "chess", LabelZH: "发条装置", LabelEN: "Clockworks", Default: "default", Choices: worldgenFreqChoices},
			{Key: "walrus", LabelZH: "海象营地", LabelEN: "MacTusk Camps", Default: "default", Choices: worldgenFreqChoices},
			{Key: "merm", LabelZH: "漏雨的小屋", LabelEN: "Leaky Shack", Default: "default", Choices: worldgenFreqChoices},
			{Key: "spiders", LabelZH: "蜘蛛巢", LabelEN: "Spider Dens", Default: "default", Choices: worldgenFreqChoices},
			{Key: "tentacles", LabelZH: "触手", LabelEN: "Tentacles", Default: "default", Choices: worldgenFreqChoices},
			{Key: "tallbirds", LabelZH: "高脚鸟", LabelEN: "Tallbirds", Default: "default", Choices: worldgenFreqChoices},
		},
	},
}

// cavesSchema follows the exact DST client section, group, and item ordering
// for cave sub-worlds (location="cave", is_master_world=false).
var cavesSchema = []WorldOptionGroup{
	// === World Settings (世界规则) ===
	{
		ID: "settings_misc", Section: "settings", NameZH: "世界", NameEN: "World",
		Options: []WorldOptionField{
			{Key: "earthquakes", LabelZH: "地震", LabelEN: "Earthquakes", Default: "default", Choices: freqChoices},
			{Key: "wormattacks", LabelZH: "洞穴蠕虫攻击", LabelEN: "Cave Worm Attacks", Default: "default", Choices: freqChoices},
			{Key: "acidrain_enabled", LabelZH: "酸雨", LabelEN: "Acid Rain", Default: "always", Choices: enabledDisabledChoices},
			{Key: "weather", LabelZH: "雨", LabelEN: "Rain", Default: "default", Choices: freqChoices},
		},
	},
	{
		ID: "settings_resources", Section: "settings", NameZH: "资源再生", NameEN: "Resource Regrowth",
		Options: []WorldOptionField{
			{Key: "regrowth", LabelZH: "再生速度", LabelEN: "Regrowth Multiplier", Default: "default", Order: 1, Choices: regrowthChoices},
		},
	},
	{
		ID: "settings_giants", Section: "settings", NameZH: "巨兽", NameEN: "Giants",
		Options: []WorldOptionField{
			{Key: "daywalker", LabelZH: "梦魇疯猪", LabelEN: "Nightmare Werepig", Default: "default", Choices: freqChoices},
			{Key: "toadstool", LabelZH: "毒菌蟾蜍", LabelEN: "Toadstool", Default: "default", Choices: freqChoices},
		},
	},
	// === World Generation (世界生成) ===
	{
		ID: "worldgen_misc", Section: "worldgen", NameZH: "世界", NameEN: "World",
		Options: []WorldOptionField{
			{Key: "world_size", LabelZH: "世界大小", LabelEN: "World Size", Default: "default", Order: 3, Choices: worldSizeChoices},
			{Key: "branching", LabelZH: "分支", LabelEN: "Branches", Default: "default", Order: 4, Choices: branchingChoices},
			{Key: "loop", LabelZH: "环形", LabelEN: "Loops", Default: "default", Order: 5, Choices: loopChoices},
		},
	},
	{
		ID: "worldgen_resources", Section: "worldgen", NameZH: "资源", NameEN: "Resources",
		Options: []WorldOptionField{
			{Key: "rock", LabelZH: "巨石", LabelEN: "Boulders", Default: "default", Choices: worldgenFreqChoices},
			{Key: "fern", LabelZH: "洞穴蕨类", LabelEN: "Cave Ferns", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flint", LabelZH: "燧石", LabelEN: "Flint", Default: "default", Choices: worldgenFreqChoices},
			{Key: "reeds", LabelZH: "芦苇", LabelEN: "Reeds", Default: "default", Choices: worldgenFreqChoices},
			{Key: "lichen", LabelZH: "苔藓", LabelEN: "Lichen", Default: "default", Choices: worldgenFreqChoices},
			{Key: "flower_cave", LabelZH: "荧光花", LabelEN: "Light Flowers", Default: "default", Choices: worldgenFreqChoices},
			{Key: "mushtree", LabelZH: "蘑菇树", LabelEN: "Mushroom Trees", Default: "default", Choices: worldgenFreqChoices},
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
			{Key: "fissure", LabelZH: "梦魇裂隙", LabelEN: "Nightmare Fissures", Default: "default", Choices: worldgenFreqChoices},
			{Key: "cave_spiders", LabelZH: "蛛网岩", LabelEN: "Spilagmites", Default: "default", Choices: worldgenFreqChoices},
			{Key: "bats", LabelZH: "蝙蝠", LabelEN: "Bats", Default: "default", Choices: worldgenFreqChoices},
		},
	},
}
