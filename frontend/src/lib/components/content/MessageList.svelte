<!-- ABOUTME: Renders an array of messages with content rendering components. -->
<!-- ABOUTME: Groups consecutive tool-only messages and passes context through to each component. -->
<script lang="ts">
  import type { MessageWithToolCalls } from "../../api/types.js";
  import { buildDisplayItems } from "../../utils/display-items.js";
  import MessageContent from "./MessageContent.svelte";
  import ToolCallGroup from "./ToolCallGroup.svelte";

  interface Props {
    messages: MessageWithToolCalls[];
    developerName?: string;
    agentName?: string;
    highlightOrdinal?: number | null;
  }

  let { messages, developerName, agentName, highlightOrdinal = null }: Props = $props();

  let displayItems = $derived(buildDisplayItems(messages));

  function groupKey(item: ReturnType<typeof buildDisplayItems>[number]): number {
    if (item.kind === "message") return item.message.ordinal;
    return item.messages[0]!.ordinal;
  }

  function groupHighlighted(item: ReturnType<typeof buildDisplayItems>[number]): boolean {
    if (highlightOrdinal == null) return false;
    if (item.kind === "message") return item.message.ordinal === highlightOrdinal;
    return item.messages.some(m => m.ordinal === highlightOrdinal);
  }
</script>

<div class="message-list">
  {#each displayItems as item (groupKey(item))}
    {#if item.kind === "message"}
      <div data-ordinal={item.message.ordinal}>
        <MessageContent message={item.message} {developerName} {agentName} expandTools={highlightOrdinal === item.message.ordinal} highlight={highlightOrdinal === item.message.ordinal} />
      </div>
    {:else}
      <div data-ordinal={item.messages[0]?.ordinal}>
        <ToolCallGroup messages={item.messages} timestamp={item.timestamp} expandTools={groupHighlighted(item)} />
      </div>
    {/if}
  {/each}
</div>

<style>
  .message-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px 0;
  }

</style>
