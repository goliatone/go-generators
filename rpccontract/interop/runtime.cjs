const assert = require('node:assert/strict');
const fs = require('node:fs');
const { createDispatcher, createEndpoints } = require('./contracts.js');
const wire = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));

async function main() {
  let calls = 0;
  const expected = wire.dispatchRequest;
  const dispatch = createDispatcher(async (request) => {
    calls++;
    assert.equal(request.data.command_id, expected.data.command_id);
    assert.equal(Object.hasOwn(request.data, 'name'), false);
    assert.deepEqual(request.data.payload, expected.data.payload);
    assert.deepEqual(request.data.ids, expected.data.ids);
    assert.deepEqual(request.data.options, expected.data.options);
    assert.deepEqual(request.meta, expected.meta);
    return wire.dispatchInline;
  });
  const response = await dispatch(expected.data.command_id, wire.input, { ids: expected.data.ids, options: expected.data.options, meta: expected.meta });
  assert.equal(calls, 1);
  assert.strictEqual(response, wire.dispatchInline);
  assert.equal(response.data.receipt.Accepted, true);
  assert.equal(response.data.receipt.CommandID, 'example.echo');
  assert.equal(response.data.result.value, false);
  assert.equal(response.data.result.count, 0);
  assert.equal(response.data.result.text, '');
  assert.equal(response.data.result.data, 'application data');
  assert.equal(response.data.result.result, 'application result');
  for (const name of ['dispatchQueued', 'dispatchFailure']) {
    const fn = createDispatcher(async () => wire[name]);
    assert.strictEqual(await fn('example.echo', wire.input), wire[name]);
  }
  assert.equal(Object.hasOwn(wire.dispatchQueued.data, 'result'), false);
  assert.equal(wire.dispatchQueued.data.receipt.Mode, 'queued');
  assert.equal(wire.dispatchQueued.data.receipt.DispatchID, 'dispatch-1');
  assert.equal(wire.dispatchFailure.data.receipt.Accepted, false);
  assert.equal(wire.dispatchFailure.data.validation_errors[0].path, 'count');
  const present = { presentNull: ['example.nullable', null], presentFalse: ['example.boolean', false], presentZero: ['example.number', 0], presentEmpty: ['example.string', ''], presentSlice: ['example.slice', []], presentMap: ['example.map', {}] };
  const omitted = { omittedFalse: 'example.boolean', omittedZero: 'example.number', omittedEmpty: 'example.string', omittedNil: 'example.slice', omittedSlice: 'example.slice', omittedMap: 'example.map' };
  for (const [name, method] of Object.entries(omitted)) {
    const fn = createEndpoints(async (actual, request) => { assert.equal(actual, method); assert.deepEqual(request, wire.request); return wire[name]; });
    const result = await fn(method, wire.input);
    assert.strictEqual(result, wire[name]);
    assert.equal(Object.hasOwn(result, 'data'), false);
  }
  for (const [name, [method, value]] of Object.entries(present)) {
    const fn = createEndpoints(async () => wire[name]);
    const result = await fn(method, wire.input);
    assert.strictEqual(result, wire[name]);
    assert.equal(Object.hasOwn(result, 'data'), true);
    assert.deepEqual(result.data, value);
  }
  const edges = createEndpoints(async (method, request) => {
    assert.equal(method, 'example.wire_edges');
    assert.deepEqual(request, wire.wireEdgesRequest);
    return wire.wireEdgesResponse;
  });
  const edgeResponse = await edges('example.wire_edges', wire.wireEdgesInput);
  assert.strictEqual(edgeResponse, wire.wireEdgesResponse);
  assert.equal(edgeResponse.data['-'], 7);
  assert.equal(edgeResponse.data.number, '9007199254740993');
  assert.equal(Object.hasOwn(edgeResponse.data, 'Ignored'), false);
  const endpoint = createEndpoints(async () => wire.object);
  assert.strictEqual(await endpoint('example.echo', wire.input), wire.object);
  const failure = Object.assign(new Error(wire.error.error.message), wire.error.error);
  await assert.rejects(createDispatcher(async () => { throw failure; })('example.echo', wire.input), error => error === failure);
  await assert.rejects(createEndpoints(async () => { throw failure; })('example.echo', wire.input), error => error === failure);
  assert.equal(wire.input.count, Number.MAX_SAFE_INTEGER);
  assert.equal(wire.inputMin.count, Number.MIN_SAFE_INTEGER);
  assert.equal(wire.input.wide, '18446744073709551615');
  console.log('PASS: actual-Go callbacks, IDs/options/receipts, errors, omission/null/falsy and domain data/result');
}
main().catch(error => { console.error(error); process.exitCode = 1; });
