import Dexie, { type Table } from "dexie";
import type {
  UserConfigGateway,
  UserConfigGetInput,
  UserConfigSetInput,
  UserConfigUserId
} from "../types";

// user_config 表记录：按用户 + 配置键组织 KV 数据。
interface UserConfigRecord {
  id?: number;
  userId: UserConfigUserId;
  key: string;
  value: unknown;
  updatedAt: string;
}

// Dexie 数据库定义：默认数据库名固定为 plaindoc_user_config。
class UserConfigDatabase extends Dexie {
  readonly userConfigTable: Table<UserConfigRecord, number>;

  constructor() {
    super("plaindoc_user_config");

    // 建表结构：
    // - 主键：自增 id
    // - 复合唯一查询键：[userId+key]
    // - 普通索引：userId / key / updatedAt
    this.version(1).stores({
      user_config: "++id,[userId+key],userId,key,updatedAt"
    });

    this.userConfigTable = this.table("user_config");
  }
}

let singletonDatabase: UserConfigDatabase | null = null;

function nowIso(): string {
  return new Date().toISOString();
}

// 获取数据库单例：避免重复 open 造成额外连接开销。
function getDatabase(): UserConfigDatabase {
  if (!singletonDatabase) {
    singletonDatabase = new UserConfigDatabase();
  }
  return singletonDatabase;
}

// 读取单条配置：由 userId + key 精确定位。
async function findConfigRecord(input: UserConfigGetInput): Promise<UserConfigRecord | undefined> {
  return getDatabase()
    .userConfigTable
    .where("[userId+key]")
    .equals([input.userId, input.key])
    .first();
}

// IndexedDB 配置网关：提供统一 get/set API 给上层业务使用。
export function createIndexedDbUserConfigGateway(): UserConfigGateway {
  return {
    async getValue<T = unknown>(input: UserConfigGetInput): Promise<T | null> {
      const matched = await findConfigRecord(input);
      return matched ? (matched.value as T) : null;
    },
    async setValue<T = unknown>(input: UserConfigSetInput & { value: T }): Promise<void> {
      const table = getDatabase().userConfigTable;
      const matched = await findConfigRecord(input);

      // 命中已有配置时执行更新，保持 id 不变。
      if (typeof matched?.id === "number") {
        await table.update(matched.id, {
          value: input.value,
          updatedAt: nowIso()
        });
        return;
      }

      // 首次写入时新增记录。
      await table.add({
        userId: input.userId,
        key: input.key,
        value: input.value,
        updatedAt: nowIso()
      });
    }
  };
}
