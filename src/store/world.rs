//! 世界视图：当前节点与出口。

use crate::protocol::msg::MapView;

#[derive(Default)]
pub struct WorldState {
    map: Option<MapView>,
}

impl WorldState {
    pub fn set(&mut self, m: MapView) {
        self.map = Some(m);
    }

    pub fn get(&self) -> Option<&MapView> {
        self.map.as_ref()
    }
}
