import test from 'node:test';
import assert from 'node:assert/strict';
import {groupListeningPorts,tcpPortChoices,portTargetKey} from '../src/ports-model.mjs';

test('same port combines IPv4, IPv6, TCP, UDP and all owning processes',()=>{
  const rows=[
    {Port:8080,Protocol:'tcp',Address:'0.0.0.0',PID:1},
    {Port:53,Protocol:'udp',Address:'127.0.0.53',PID:3},
    {Port:8080,Protocol:'tcp',Address:'::',PID:2},
    {Port:8080,Protocol:'udp',Address:'0.0.0.0',PID:1}
  ];
  const groups=groupListeningPorts(rows);
  assert.deepEqual(groups.map(group=>group.Port),[53,8080]);
  assert.equal(groups[1].Entries.length,3);
  assert.deepEqual(groups[1].Entries.map(entry=>entry.index),[0,2,3]);
  assert.equal(groups[1].Entries[1].port,rows[2]);
});

test('TCP choices deduplicate shared sockets but retain distinct bind addresses',()=>{
  const rows=[
    {Port:80,Protocol:'tcp',Address:'0.0.0.0',PID:1},
    {Port:80,Protocol:'tcp',Address:'0.0.0.0',PID:2},
    {Port:80,Protocol:'tcp',Address:'127.0.0.1',PID:1},
    {Port:80,Protocol:'tcp',Address:'192.168.1.9',PID:4},
    {Port:80,Protocol:'tcp',Address:'::',PID:1},
    {Port:80,Protocol:'udp',Address:'0.0.0.0',PID:8}
  ];
  const choices=tcpPortChoices(groupListeningPorts(rows)[0]);
  assert.deepEqual(choices.map(choice=>choice.index),[0,3,4]);
  assert.deepEqual(choices.map(choice=>portTargetKey(choice.port)),['127.0.0.1:80','192.168.1.9:80','[::1]:80']);
});

test('UDP-only groups expose no TCP tunnel action',()=>{
  const groups=groupListeningPorts([{Port:53,Protocol:'udp',Address:'::',PID:4}]);
  assert.deepEqual(tcpPortChoices(groups[0]),[]);
  assert.deepEqual(groupListeningPorts([]),[]);
});
