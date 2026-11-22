//! 角色状态：全量快照 + vital 增量补丁。

use crate::protocol::msg::{PlayerView, VitalView};

#[derive(Default)]
pub struct PlayerState {
    view: Option<PlayerView>,
}

impl PlayerState {
    pub fn set(&mut self, v: PlayerView) {
        self.view = Some(v);
    }

    pub fn get(&self) -> Option<&PlayerView> {
        self.view.as_ref()
    }

    /// `vital` 只覆盖它自己带的字段。
    pub fn patch_vitals(&mut self, v: &VitalView) {
        if let Some(p) = self.view.as_mut() {
            p.hp = v.hp;
            p.mp = v.mp;
            p.exp = v.exp;
            p.exp_need = v.exp_need;
            p.stamina = v.stamina;
            p.cultivating = v.cultivating;
            p.buffs = v.buffs.clone();
        }
    }
}
