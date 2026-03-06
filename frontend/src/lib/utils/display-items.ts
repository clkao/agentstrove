// ABOUTME: Groups consecutive tool-only assistant messages into compact display items.
// ABOUTME: Transforms flat message arrays into DisplayItem arrays for efficient rendering.

import type { MessageWithToolCalls } from "../api/types.js";
import { isToolOnly } from "./content-parser.js";

export interface MessageItem {
  kind: "message";
  message: MessageWithToolCalls;
}

export interface ToolGroupItem {
  kind: "tool-group";
  messages: MessageWithToolCalls[];
  timestamp: string | null;
}

export type DisplayItem = MessageItem | ToolGroupItem;

export function buildDisplayItems(messages: MessageWithToolCalls[]): DisplayItem[] {
  const items: DisplayItem[] = [];
  let toolAcc: MessageWithToolCalls[] = [];

  for (const msg of messages) {
    if (isToolOnly(msg)) {
      toolAcc.push(msg);
    } else {
      if (toolAcc.length > 0) {
        items.push({
          kind: "tool-group",
          messages: toolAcc,
          timestamp: toolAcc[0]!.timestamp,
        });
        toolAcc = [];
      }
      items.push({ kind: "message", message: msg });
    }
  }

  if (toolAcc.length > 0) {
    items.push({
      kind: "tool-group",
      messages: toolAcc,
      timestamp: toolAcc[0]!.timestamp,
    });
  }

  return items;
}
