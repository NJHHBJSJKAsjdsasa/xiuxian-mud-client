#![allow(dead_code)] // 协议常量与视图字段按服务端 05 全量声明，未用到的属于后续里程碑。

//! 协议类型：字段名与服务端 `docs/05-PROTOCOL.md` 一致，不改名。
//!
//! 反序列化一律宽松：服务端新增字段不影响，缺字段用默认值。

pub mod action;
pub mod err;
pub mod frame;
pub mod msg;

/// 客户端协议版本，`hello.d.proto` 回带。
pub const PROTO: i32 = 1;
