import test from 'node:test';
import assert from 'node:assert/strict';
import {terminalSkills,skillInput,skillDraft} from '../src/terminal-skills.mjs';

test('each built-in prompt can be pasted without submitting or terminal control characters',()=>{
  assert.equal(new Set(terminalSkills.map(item=>item.id)).size,terminalSkills.length);
  for(const skill of terminalSkills){
    const text=skillInput({prompt:skillDraft(skill.id)});
    assert.equal(text,skillInput(skill));
    assert.doesNotMatch(text,/[\x00-\x1f\x7f-\x9f]/);
    assert.ok(text.endsWith('我的任务： '));
  }
});

test('switching skills preserves the task and custom input removes the template',()=>{
  const task='打开我的网站';
  const first=skillDraft('client-browser',task);
  const next=skillDraft('client-run',first,'client-browser');
  assert.equal(next,terminalSkills.find(s=>s.id==='client-run').prompt+'\n'+task);
  assert.equal(skillDraft('',next,'client-run'),task);
  assert.equal(skillDraft('unknown',task),task);
});

test('prompt normalization strips control sequences and line endings',()=>{
  assert.equal(skillInput({prompt:'a\r\nb\t\x03\x1bc'}),'a b c ');
});
