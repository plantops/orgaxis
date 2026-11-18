"""Small, auditable temporal reporting core. Synthetic demo; not a production HRIS."""
from __future__ import annotations

import json
import os
import sqlite3
import uuid
from contextlib import contextmanager
from datetime import date, datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent
DB_PATH = Path(os.getenv('HR_DB_PATH', str(ROOT / 'demo.sqlite3')))
TENANT = 'tenant-demo'


def utc_now():
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def uid(prefix):
    return prefix + '-' + uuid.uuid4().hex[:14]


def date_iso(s: str) -> str:
    try:
        result = date.fromisoformat(s).isoformat()
    except (ValueError, TypeError):
        raise ValueError('as_of/effective_date must be YYYY-MM-DD')
    return result


def connect(db_path=None):
    path = str(db_path or DB_PATH)
    con = sqlite3.connect(path, timeout=15, isolation_level=None)
    con.row_factory = sqlite3.Row
    con.execute('PRAGMA foreign_keys = ON')
    con.execute('PRAGMA busy_timeout = 15000')
    con.execute('PRAGMA journal_mode = WAL')
    return con


def setup(con):
    con.executescript((ROOT / 'schema.sql').read_text(encoding='utf-8'))


def one(con, sql, args=()):
    r = con.execute(sql, args).fetchone()
    return dict(r) if r else None


def rows(con, sql, args=()):
    return [dict(x) for x in con.execute(sql, args).fetchall()]


def active(col=''):
    p = col + '.' if col else ''
    return f'{p}valid_from <= ? AND ({p}valid_to IS NULL OR {p}valid_to > ?)'


class DomainError(Exception):
    def __init__(self, status, message):
        self.status = status
        self.message = message
        super().__init__(message)


def person_assignment(con, person_id, at):
    data = rows(con, f'''SELECT a.*, p.code AS position_code, p.title AS position_title
        FROM assignment a JOIN position p ON p.id=a.position_id
        WHERE a.tenant_id=? AND a.person_id=? AND a.assignment_type='primary'
          AND {active('a')}''', (TENANT, person_id, at, at))
    if len(data) > 1:
        raise DomainError(409, 'More than one active primary assignment')
    return data[0] if data else None


def _manager_from_position(con, position_id, at):
    links = rows(con, f'''SELECT r.*, p.code AS supervisor_code
        FROM position_reporting r JOIN position p ON p.id=r.supervisor_position_id
        WHERE r.tenant_id=? AND r.position_id=? AND r.relation_type='LINE' AND {active('r')}''',
        (TENANT, position_id, at, at))
    if len(links) > 1:
        return {'status': 'ambiguous_reporting', 'supervisor_position_id': None, 'manager': None}
    if not links:
        return {'status': 'root', 'supervisor_position_id': None, 'manager': None}
    link = links[0]
    staff = rows(con, f'''SELECT a.person_id, p.display_name FROM assignment a
        JOIN person p ON p.id=a.person_id WHERE a.tenant_id=? AND a.position_id=?
        AND a.assignment_type IN ('primary','acting') AND {active('a')}''',
        (TENANT, link['supervisor_position_id'], at, at))
    if not staff:
        status = 'vacant_supervisor'
    elif len(staff) > 1:
        status = 'ambiguous_supervisor'
    else:
        status = 'resolved'
    return {'status': status, 'supervisor_position_id': link['supervisor_position_id'],
            'supervisor_position_code': link['supervisor_code'],
            'manager': staff[0] if status == 'resolved' else None}


def manager_for_person(con, person_id, at):
    date_iso(at)
    emp = one(con, 'SELECT id, display_name FROM person WHERE tenant_id=? AND id=?', (TENANT, person_id))
    if emp is None:
        raise DomainError(404, 'Unknown person')
    ass = person_assignment(con, person_id, at)
    if not ass:
        return {'person_id': person_id, 'as_of': at, 'status': 'unassigned', 'manager': None}
    found = _manager_from_position(con, ass['position_id'], at)
    return {'person_id': person_id, 'as_of': at, 'position_code': ass['position_code'], **found}


def chain_for_person(con, person_id, at, max_levels=20):
    if max_levels > 50 or max_levels < 1:
        raise DomainError(422, 'max_levels must be 1..50')
    first = manager_for_person(con, person_id, at)
    if first['status'] == 'unassigned':
        return {'person_id': person_id, 'as_of': at, 'status': 'unassigned', 'chain': []}
    chain = []
    current_person = person_id
    visited = {current_person}
    for _ in range(max_levels):
        step = manager_for_person(con, current_person, at)
        if step['status'] != 'resolved':
            return {'person_id': person_id, 'as_of': at, 'status': step['status'],
                    'chain': chain, 'terminal': step}
        manager = step['manager']
        if manager['person_id'] in visited:
            raise DomainError(409, 'Cycle in reporting chain')
        chain.append({'level': len(chain) + 1,
                      'person_id': manager['person_id'],
                      'display_name': manager['display_name'],
                      'position_code': step['supervisor_position_code']})
        visited.add(manager['person_id'])
        current_person = manager['person_id']
    return {'person_id': person_id, 'as_of': at, 'status': 'limit_reached', 'chain': chain}


def blank_org(con, at):
    date_iso(at)
    units = rows(con, '''SELECT id,code,name,unit_type,legal_entity_id
        FROM org_unit WHERE tenant_id=? AND created_at<=? ORDER BY code''', (TENANT, at))
    rels = rows(con, f'''SELECT r.parent_id, r.child_id, r.relation_type FROM org_unit_relation r
        WHERE r.tenant_id=? AND {active('r')} ORDER BY r.child_id''', (TENANT, at, at))
    positions = rows(con, f'''SELECT p.id,p.code,p.title,p.holder_limit,pl.org_unit_id,
        r.supervisor_position_id FROM position p
        JOIN position_placement pl ON pl.position_id=p.id AND pl.tenant_id=p.tenant_id
        LEFT JOIN position_reporting r ON r.position_id=p.id AND r.tenant_id=p.tenant_id
            AND r.relation_type='LINE' AND {active('r')}
        WHERE p.tenant_id=? AND p.created_at<=? AND {active('pl')}
        ORDER BY p.code''', (at, at, TENANT, at, at, at))
    return {'as_of': at, 'units': units, 'org_edges': rels, 'positions': positions,
            'unit_count': len(units), 'position_count': len(positions)}


def occupied_org(con, at, limit=100, offset=0):
    date_iso(at)
    if limit < 1 or limit > 1000 or offset < 0:
        raise DomainError(422, 'limit must be 1..1000, offset >= 0')
    staff = rows(con, f'''SELECT a.person_id, per.display_name, p.code AS position_code,
        a.assignment_type, pl.org_unit_id
        FROM assignment a JOIN person per ON per.id=a.person_id
        JOIN position p ON p.id=a.position_id
        LEFT JOIN position_placement pl ON pl.position_id=p.id AND {active('pl')}
        WHERE a.tenant_id=? AND {active('a')} ORDER BY a.person_id LIMIT ? OFFSET ?''',
        (at, at, TENANT, at, at, limit, offset))
    count = con.execute(f'''SELECT COUNT(*) FROM assignment a WHERE a.tenant_id=?
         AND {active('a')}''', (TENANT, at, at)).fetchone()[0]
    return {'as_of': at, 'total': count, 'limit': limit, 'offset': offset, 'assignments': staff}


def workflow_resolve(con, subject_id, at, policy, levels):
    if policy != 'LINE_MANAGER':
        raise DomainError(422, 'Only LINE_MANAGER policy is implemented in demo')
    if levels < 1 or levels > 10:
        raise DomainError(422, 'levels must be 1..10')
    result = chain_for_person(con, subject_id, at, max_levels=max(20, levels))
    if len(result['chain']) >= levels:
        target = result['chain'][levels - 1]
        return {'as_of': at, 'policy': policy, 'requested_level': levels,
                'subject_id': subject_id, 'status': 'resolved', 'assignee_id': target['person_id'],
                'position_code': target['position_code'], 'full_chain': result['chain']}
    return {'as_of': at, 'policy': policy, 'requested_level': levels,
            'subject_id': subject_id, 'status': 'unresolved', 'reason': result['status'],
            'assignee_id': None, 'full_chain': result['chain']}


def _ensure_person_and_position(con, person_id, pos_code, at):
    if one(con, 'SELECT id FROM person WHERE tenant_id=? AND id=?', (TENANT, person_id)) is None:
        raise DomainError(404, 'Unknown person')
    pos = one(con, '''SELECT p.*, pl.org_unit_id, u.legal_entity_id FROM position p
        JOIN position_placement pl ON pl.position_id=p.id AND pl.tenant_id=p.tenant_id
        JOIN org_unit u ON u.id=pl.org_unit_id
        WHERE p.tenant_id=? AND p.code=? AND p.created_at<=? AND ''' + active('pl'),
        (TENANT, pos_code, at, at, at))
    if pos is None:
        raise DomainError(404, 'Target position not active')
    return pos


def move_assignment(con, person_id, target_pos_code, effective_date, reason='transfer'):
    effective_date = date_iso(effective_date)
    with _transaction(con):
        pos = _ensure_person_and_position(con, person_id, target_pos_code, effective_date)
        old = person_assignment(con, person_id, effective_date)
        if old is None:
            raise DomainError(409, 'No active primary assignment')
        if old['position_id'] == pos['id']:
            raise DomainError(409, 'Already assigned to target position')
        if effective_date <= old['valid_from']:
            raise DomainError(409, 'Move date must be after current assignment start')
        if old['valid_to'] is not None:
            raise DomainError(409, 'Move into a historical segment is not supported')
        future = one(con, """SELECT id FROM assignment WHERE tenant_id=? AND person_id=?
             AND assignment_type='primary' AND valid_from>=?""", (TENANT, person_id, effective_date))
        if future:
            raise DomainError(409, 'Future assignment exists; use a planned change workflow')
        legal = one(con, '''SELECT legal_entity_id FROM employment_episode WHERE id=? AND tenant_id=?''',
                    (old['employment_id'], TENANT))
        if legal['legal_entity_id'] != pos['legal_entity_id']:
            raise DomainError(409, 'Cross-entity moves require an employment mobility case')
        occupied = con.execute(f'''SELECT COUNT(*) FROM assignment WHERE tenant_id=? AND position_id=?
             AND {active()}''', (TENANT, pos['id'], effective_date, effective_date)).fetchone()[0]
        if occupied >= pos['holder_limit']:
            raise DomainError(409, 'Target position is at holder capacity')
        change_id = uid('CS')
        con.execute('''INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,recorded_at)
          VALUES (?,?,?,?,?,'published',?)''', (change_id,TENANT,'workforce',f'Move {person_id}',effective_date,utc_now()))
        con.execute('UPDATE assignment SET valid_to=? WHERE id=?', (effective_date,old['id']))
        con.execute('''INSERT INTO assignment(id,tenant_id,person_id,employment_id,position_id,
          assignment_type,fte,valid_from,change_set_id)
          VALUES (?,?,?,?,?,'primary',1,?,?)''',
          (uid('ASS'),TENANT,person_id,old['employment_id'],pos['id'],effective_date,change_id))
        con.execute('''INSERT INTO hr_event(id,tenant_id,person_id,event_type,effective_date,status,change_set_id,details)
          VALUES (?,?,?,?,?,'published',?,?)''',
          (uid('EV'),TENANT,person_id,reason,effective_date,change_id,
           json.dumps({'from': old['position_code'], 'to': target_pos_code})))
        _audit(con, 'assignment.move', person_id, {'from': old['position_code'], 'to': target_pos_code,
                                                     'effective_date': effective_date, 'change_set': change_id})
    return {'change_set_id': change_id, 'person_id': person_id, 'effective_date': effective_date,
            'from_position': old['position_code'], 'to_position': target_pos_code}


def _has_reporting_cycle(con, pos_id, parent_id, at):
    seen = set()
    current = parent_id
    while current:
        if current == pos_id or current in seen:
            return True
        seen.add(current)
        rel = rows(con, f'''SELECT supervisor_position_id FROM position_reporting
         WHERE tenant_id=? AND position_id=? AND relation_type='LINE' AND {active()}''',
         (TENANT, current, at, at))
        if len(rel) > 1:
            raise DomainError(409, 'Ambiguous reporting before change')
        current = rel[0]['supervisor_position_id'] if rel else None
    return False


def reporting_change(con, position_code, new_supervisor_code, effective_date):
    effective_date = date_iso(effective_date)
    with _transaction(con):
        position = one(con, 'SELECT id FROM position WHERE tenant_id=? AND code=? AND created_at<=?',
                       (TENANT, position_code, effective_date))
        parent = one(con, 'SELECT id FROM position WHERE tenant_id=? AND code=? AND created_at<=?',
                     (TENANT, new_supervisor_code, effective_date))
        if not position or not parent:
            raise DomainError(404, 'Unknown/inactive position')
        if position['id'] == parent['id']:
            raise DomainError(409, 'Position cannot report to itself')
        current = rows(con, f'''SELECT * FROM position_reporting WHERE tenant_id=? AND position_id=?
             AND relation_type='LINE' AND {active()}''', (TENANT, position['id'], effective_date, effective_date))
        if len(current) > 1:
            raise DomainError(409, 'Ambiguous existing reporting')
        if current and current[0]['supervisor_position_id'] == parent['id']:
            raise DomainError(409, 'Reporting already set to this supervisor')
        if current and (effective_date <= current[0]['valid_from'] or current[0]['valid_to'] is not None):
            raise DomainError(409, 'Historical segment editing is not supported')
        future = one(con, """SELECT id FROM position_reporting WHERE tenant_id=? AND position_id=?
              AND relation_type='LINE' AND valid_from>=?""", (TENANT, position['id'], effective_date))
        if future:
            raise DomainError(409, 'Future reporting change already exists')
        if _has_reporting_cycle(con, position['id'], parent['id'], effective_date):
            raise DomainError(409, 'Reporting cycle')
        change_id = uid('CS')
        con.execute('''INSERT INTO change_set(id,tenant_id,domain,title,effective_date,status,recorded_at)
          VALUES (?,?,?,?,?,'published',?)''',
          (change_id,TENANT,'org_structure','Change reporting line',effective_date,utc_now()))
        if current:
            con.execute('UPDATE position_reporting SET valid_to=? WHERE id=?', (effective_date,current[0]['id']))
        con.execute('''INSERT INTO position_reporting(id,tenant_id,position_id,supervisor_position_id,
         relation_type,valid_from,change_set_id) VALUES (?,?,?,?,'LINE',?,?)''',
         (uid('REL'),TENANT,position['id'],parent['id'],effective_date,change_id))
        _audit(con,'org.reporting_change', position_code,
               {'to': new_supervisor_code, 'effective_date': effective_date, 'change_set': change_id})
    return {'change_set_id': change_id, 'position': position_code,
            'supervisor_position': new_supervisor_code, 'effective_date': effective_date}


def _audit(con, event, target, payload):
    con.execute('INSERT INTO audit_log(tenant_id,happened_at,actor,event_name,target,payload) VALUES (?,?,?,?,?,?)',
                (TENANT,utc_now(),'demo_api',event,target,json.dumps(payload,ensure_ascii=False)))

@contextmanager
def _transaction(con):
    con.execute('BEGIN IMMEDIATE')
    try:
        yield
        con.execute('COMMIT')
    except Exception:
        con.execute('ROLLBACK')
        raise


def direct_reports(con, manager_id, at, limit=100, offset=0):
    date_iso(at)
    if limit < 1 or limit > 1000 or offset < 0:
        raise DomainError(422, 'limit must be 1..1000 and offset >= 0')
    manager = person_assignment(con, manager_id, at)
    if not manager:
        raise DomainError(404, 'Manager not found or unassigned')
    args = (TENANT,manager['position_id'],at,at,TENANT,at,at)
    # Positions, not people, determine the authoritative reporting relationship.
    q = f'''SELECT a.person_id,p.display_name, pos.code AS position_code
     FROM position_reporting r JOIN assignment a ON a.position_id=r.position_id
     JOIN person p ON p.id=a.person_id JOIN position pos ON pos.id=a.position_id
     WHERE r.tenant_id=? AND r.supervisor_position_id=? AND r.relation_type='LINE'
     AND {active('r')} AND a.tenant_id=? AND a.assignment_type='primary' AND {active('a')}'''
    all_rows = rows(con,q+' ORDER BY a.person_id',args)
    return {'manager_id':manager_id,'as_of':at,'total':len(all_rows),
            'limit':limit,'offset':offset,'reports':all_rows[offset:offset+limit]}


def org_changes(con):
    return rows(con, '''SELECT id,title,effective_date,status,scenario
     FROM change_set WHERE tenant_id=? AND domain='org_structure' ORDER BY effective_date,id''', (TENANT,))
