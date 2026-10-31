import { describe, expect, it } from 'vitest';
import type { IssueComment } from '../api/types';
import { groupInline, labelTextColor, lineKey, reviewVerdicts } from './issues';

const c = (o: Partial<IssueComment>): IssueComment => ({ id: 1, author: 'ann', body: '', kind: 'comment', created_at: '', updated_at: '', ...o });

describe('labelTextColor', () => {
  it('uses dark text on light colours and light text on dark ones', () => {
    expect(labelTextColor('#ffffff')).toBe('#000000');
    expect(labelTextColor('#fbca04')).toBe('#000000');
    expect(labelTextColor('#1d4f91')).toBe('#ffffff');
    expect(labelTextColor('bogus')).toBe('#ffffff');
  });
});

describe('groupInline', () => {
  it('groups only comments attached to a line', () => {
    const m = groupInline([
      c({ id: 1, kind: 'review', path: 'a.go', line: 3, side: 'new' }),
      c({ id: 2, kind: 'review', path: 'a.go', line: 3, side: 'new' }),
      c({ id: 3, kind: 'review', path: 'a.go', line: 3, side: 'old' }),
      c({ id: 4 }),
    ]);
    expect(m.get(lineKey('a.go', 'new', 3))?.map((x) => x.id)).toEqual([1, 2]);
    expect(m.get(lineKey('a.go', 'old', 3))?.map((x) => x.id)).toEqual([3]);
    expect(m.size).toBe(2);
  });
});

describe('reviewVerdicts', () => {
  it('keeps each reviewer\'s latest decision and ignores comment-only reviews', () => {
    const v = reviewVerdicts([
      c({ author: 'bob', kind: 'review', review_state: 'request_changes' }),
      c({ author: 'cat', kind: 'review', review_state: 'approve' }),
      c({ author: 'bob', kind: 'review', review_state: 'comment' }),
      c({ author: 'bob', kind: 'review', review_state: 'approve' }),
      c({ author: 'dan', kind: 'review', review_state: 'approve', path: 'x', line: 1, side: 'new' }),
    ]);
    expect(v).toEqual([{ author: 'bob', state: 'approve' }, { author: 'cat', state: 'approve' }]);
  });
});
