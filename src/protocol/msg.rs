//! 服务端视图结构。字段取自 `05-PROTOCOL.md` 的 `P-MSG-02` / `P-MSG-04` / `P-ACT-28`。

use std::collections::HashMap;

use serde::Deserialize;
use serde_json::Value;

fn f64_default() -> f64 {
    0.0
}

/// 服务端约定"字段为 null 而非省略"：集合字段遇到 null 时按空集合处理。
pub fn null_as_default<'de, D, T>(d: D) -> Result<T, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Default + serde::Deserialize<'de>,
{
    Ok(Option::<T>::deserialize(d)?.unwrap_or_default())
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct PlayerView {
    pub name: String,
    pub stage: i32,
    pub realm: String,
    pub exp: i64,
    pub exp_need: i64,
    pub hp: i64,
    pub max_hp: i64,
    pub mp: i64,
    pub max_mp: i64,
    pub atk: i64,
    pub def: i64,
    pub spd: i64,
    #[serde(default = "f64_default")]
    pub crit: f64,
    pub power: i64,
    pub stones: i64,
    pub stamina: i64,
    pub stamina_max: i64,
    pub toxin: i64,
    pub toxin_note: String,
    pub sect_id: String,
    pub faction_name: String,
    pub contrib: i64,
    pub fame: i64,
    pub fame_tier: String,
    pub merit: i64,
    pub lives: i32,
    pub guild_name: String,
    pub guild_rank: String,
    pub guild_contrib: i64,
    pub cultivating: bool,
    pub region: String,
    pub region_name: String,
    pub loc: String,
    pub loc_name: String,
    pub travel: Option<TravelView>,
    pub root: Option<RootView>,
    pub attrs: Attrs,
    pub free: i32,
    #[serde(default, deserialize_with = "null_as_default")]
    pub equip: HashMap<String, String>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub inv: HashMap<String, i64>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub skills: Vec<String>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub slots: Vec<String>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub manuals: Vec<String>,
    pub main: String,
    pub main_name: String,
    pub alchemy: Option<AlchemyView>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub buffs: Vec<BuffView>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub quests: Vec<QuestView>,
    pub stats: StatsView,
    #[serde(default, deserialize_with = "null_as_default")]
    pub bag: Vec<BagItem>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub skill_book: Vec<SkillItem>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct BagItem {
    pub id: String,
    pub n: i64,
    pub name: String,
    pub kind: String,
    pub sell: i64,
    pub equipped: bool,
    pub slot: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct SkillItem {
    pub id: String,
    pub name: String,
    pub mp: i64,
    #[serde(default = "f64_default")]
    pub power: f64,
    pub effect: String,
    pub effect_name: String,
    pub equipped: bool,
    pub slot: i32,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct TravelView {
    pub to: String,
    pub name: String,
    pub arrive_at: i64,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct RootView {
    pub kind: String,
    pub element: String,
    pub name: String,
    #[serde(default = "f64_default")]
    pub speed: f64,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Attrs {
    pub ti: i32,
    pub ling: i32,
    pub feng: i32,
    pub hu: i32,
    pub shen: i32,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct AlchemyView {
    pub level: i32,
    pub exp: i64,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct BuffView {
    pub id: String,
    pub name: String,
    pub until: i64,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct QuestView {
    pub id: String,
    pub desc: String,
    pub prog: i64,
    pub target: i64,
    pub claimed: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct StatsView {
    pub kills: i64,
    pub explores: i64,
    pub breaks: i64,
    pub pk: i64,
    pub deaths: i64,
    pub craft: i64,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct VitalView {
    pub hp: i64,
    pub mp: i64,
    pub exp: i64,
    pub exp_need: i64,
    pub stamina: i64,
    pub cultivating: bool,
    #[serde(default, deserialize_with = "null_as_default")]
    pub buffs: Vec<BuffView>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct LogLine {
    pub k: String,
    pub s: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct MapView {
    pub loc: String,
    pub name: String,
    pub desc: String,
    pub region: String,
    pub region_name: String,
    pub terrain: String,
    pub grade: Option<String>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub path: Vec<String>,
    pub explored: i32,
    pub crowd: String,
    pub world: Option<WorldEventView>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub npcs: Vec<NpcView>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub exits: Vec<ExitView>,
    pub weather: Option<String>,
    pub next: Option<String>,
    pub season: Option<String>,
    pub daypart: Option<String>,
    pub shard: Option<i64>,
    pub shards: Option<Vec<ShardView>>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct WorldEventView {
    pub id: String,
    pub name: String,
    pub until: i64,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct NpcView {
    pub id: String,
    pub name: String,
    pub title: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct ExitView {
    pub to: String,
    pub name: Option<String>,
    pub terrain: String,
    pub cost_sec: i64,
    pub cond: Option<String>,
    pub ok: bool,
    pub crowd: Option<String>,
    pub temp: Option<bool>,
    pub blocked: Option<bool>,
    pub until: Option<i64>,
    pub fee: Option<i64>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct ShardView {
    pub id: i64,
    pub name: String,
    pub crowd: String,
    pub mine: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct FoeView {
    pub i: i32,
    pub id: String,
    pub name: String,
    pub hp: i64,
    pub max_hp: i64,
    pub lvl: i32,
    pub element: String,
    #[serde(default, deserialize_with = "null_as_default")]
    pub status: Vec<String>,
    pub dead: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct AllyView {
    pub name: String,
    pub hp: i64,
    pub max_hp: i64,
    pub down: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct BattleView {
    pub id: String,
    pub kind: String,
    pub turn: i32,
    pub round: i32,
    pub state: String,
    pub enemy: Option<FoeView>,
    pub me: Option<VitalView>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub foes: Vec<FoeView>,
    #[serde(default, deserialize_with = "null_as_default")]
    pub allies: Vec<AllyView>,
    pub can_flee: bool,
    pub over: bool,
    pub won: bool,
    pub fled: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct ChatMsg {
    pub channel: String,
    pub from: String,
    pub text: String,
    pub ts: i64,
    pub npc: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct ReadyView {
    pub proto: i32,
    pub min_proto: i32,
    /// 只有"已登录、未建号"时服务端才下发；有角色时缺省，按 `player` 是否存在判断。
    pub has_char: Option<bool>,
    pub player: Option<PlayerView>,
    /// `{online, startedAt}`，不是地图视图。
    pub world: Value,
    pub presence: i64,
    #[serde(rename = "chatHist", default, deserialize_with = "null_as_default")]
    pub chat_hist: Vec<ChatMsg>,
    pub offline: Option<Value>,
    pub catalog: Value,
    pub server_now: i64,
    pub season: String,
    pub daypart: String,
}

impl ReadyView {
    pub fn has_character(&self) -> bool {
        self.has_char.unwrap_or(self.player.is_some())
    }

    pub fn online_count(&self) -> i64 {
        if self.presence > 0 {
            self.presence
        } else {
            self.world
                .get("online")
                .and_then(|v| v.as_i64())
                .unwrap_or(0)
        }
    }
}
