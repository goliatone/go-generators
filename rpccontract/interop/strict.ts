import { createDispatcher, createEndpoints } from "./contracts";
import type { CommandInputs, DispatchCallback, DispatchResponse, InvokeCallback, MethodInputs, SuccessEnvelope } from "./contracts";
import { wireEdgesInput, wireEdgesResponse, input, inputMin, dispatchInline, dispatchQueued, dispatchFailure, object, presentNull, presentFalse, presentZero, presentEmpty, presentSlice, presentMap } from "./wire";

// The host callbacks keep the canonical envelope and reject RPC errors.
declare const dispatchCallback: DispatchCallback;
declare const invokeCallback: InvokeCallback;
const dispatch = createDispatcher(dispatchCallback);
const call = createEndpoints(invokeCallback);
void dispatch("example.echo", input, { ids: ["one"], options: { Mode: "queued", CorrelationID: "corr", Delay: 1000 }, meta: { requestId: "req" } });
void dispatch("example.notify", { message: "hello" });
void call("example.echo", inputMin);
void call("example.boolean", input).then(response => { const value: boolean | null | undefined = response.data; void value; });
void call("example.number", input).then(response => { const value: number | null | undefined = response.data; void value; });
const recursive: CommandInputs["example.echo"]["shared"] = { name: "root", next: { name: "leaf", next: null } };
void recursive;
void [dispatchInline, dispatchQueued, dispatchFailure, object, presentNull, presentFalse, presentZero, presentEmpty, presentSlice, presentMap];
const receiptOnly: SuccessEnvelope<DispatchResponse<"example.notify">> = { data: { receipt: dispatchQueued.data!.receipt } };
void receiptOnly;
// @ts-expect-error unknown command
void dispatch("missing", input);
// @ts-expect-error wrong command payload
void dispatch("example.notify", input);
// @ts-expect-error public field is string-encoded uint64
void dispatch("example.echo", { ...input, wide: 18446744073709551615 });
// @ts-expect-error explicit enum
void dispatch("example.echo", { ...input, state: "bogus" });
// @ts-expect-error null is not omission for nonnullable scalar
void dispatch("example.echo", { ...input, count: null });
// @ts-expect-error canonical options use exported Go casing
void dispatch("example.echo", input, { options: { mode: "queued" } });
// @ts-expect-error ids stay separate strings
void dispatch("example.echo", input, { ids: [1] });
// @ts-expect-error unknown method
void call("missing", input);
// @ts-expect-error tuple retains fixed Go array length
const pair: MethodInputs["example.echo"]["pair"] = [1];
// @ts-expect-error receipt is required whenever dispatch data is present
const noReceipt: SuccessEnvelope<DispatchResponse<"example.echo">> = { data: { result: object.data } };
// @ts-expect-error receipt casing comes from Go
const wrongReceipt: DispatchResponse<"example.echo"> = { receipt: { accepted: true } };
// @ts-expect-error receipt-only commands cannot declare a result
const inventedResult: DispatchResponse<"example.notify"> = { receipt: dispatchQueued.data!.receipt, result: {} };
void [pair, noReceipt, wrongReceipt, inventedResult];

// Literal dash fields and actual string-encoded json.Number survive Go -> TS.
void dispatch("example.wire_edges", wireEdgesInput);
void call("example.wire_edges", wireEdgesInput).then(response => {
  const number: string | undefined = response.data?.number;
  const dash: number | undefined = response.data?.["-"];
  void [number, dash];
});
const exactWideNumber: string = wireEdgesResponse.data!.number;
void exactWideNumber;
// @ts-expect-error json.Number uses its actual Go ,string representation
void dispatch("example.wire_edges", { number: 9007199254740993, "-": 7 });
// @ts-expect-error dash-named property retains its numeric wire type
void dispatch("example.wire_edges", { number: "9007199254740993", "-": "7" });
// @ts-expect-error the exact ignore tag does not create a public property
const ignored: CommandInputs["example.wire_edges"]["Ignored"] = "not-on-wire";
// @ts-expect-error command IDs are canonical; whitespace variants are not aliases
void dispatch(" example.echo ", input);
void ignored;
