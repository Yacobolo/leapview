import { expect, test } from 'bun:test'
import type { DashboardFilterCommand, DashboardFilterExpression, DashboardFilterState } from '../../../generated/signals'
import { DashboardFilterController } from './filter-controller'

const unfiltered: DashboardFilterExpression = { kind: 'unfiltered' }

function setExpression(value: string): DashboardFilterExpression {
  return { kind: 'set', operator: 'in', values: [{ kind: 'string', value }] }
}

function state(revision: number): DashboardFilterState {
  return {
    revision,
    defaultsRevision: 'defaults',
    appliedControls: {
      state: { expression: unfiltered, resolvedExpression: unfiltered },
    },
    draftControls: {},
    dirtyBindings: [],
  }
}

test('filter controller serializes commands and rebases queued mutations after reconciliation', () => {
  const sent: DashboardFilterCommand[] = []
  const controller = new DashboardFilterController((command) => sent.push(command), () => 'mutation-id')
  controller.reconcile(state(4))

  controller.mutate('state', {
    kind: 'set',
    operator: 'in',
    values: [{ kind: 'string', value: 'CA' }],
  })
  controller.clear('state')

  expect(sent).toHaveLength(1)
  expect(sent[0]?.baseRevision).toBe(4)
  controller.reconcile(state(5))
  expect(sent).toHaveLength(2)
  expect(sent[1]?.baseRevision).toBe(5)
})

test('filter controller normalizes sparse empty collections at the signal boundary', () => {
  const sent: DashboardFilterCommand[] = []
  const controller = new DashboardFilterController((command) => sent.push(command), () => 'mutation-id')
  controller.reconcile({
    revision: 3,
    defaultsRevision: 'defaults',
    appliedControls: {},
    draftControls: {},
  } as DashboardFilterState)

  controller.mutate('state', setExpression('CA'))

  expect(sent[0]?.baseRevision).toBe(3)
  expect(controller.projected.dirtyBindings).toEqual([])
})

test('filter controller projects optimistic state without replacing unrelated controls', () => {
  const controller = new DashboardFilterController(() => {}, () => 'mutation-id')
  const current = state(2)
  current.appliedControls.category = { expression: unfiltered, resolvedExpression: unfiltered }
  controller.reconcile(current)

  controller.mutate('state', {
    kind: 'set',
    operator: 'in',
    values: [{ kind: 'string', value: 'WA' }],
  })

  expect(controller.projected.appliedControls.state.expression).toEqual({
    kind: 'set',
    operator: 'in',
    values: [{ kind: 'string', value: 'WA' }],
  })
  expect(controller.projected.appliedControls.category.expression).toEqual(unfiltered)
})

test('filter controller does not expose draft state as applied in deferred mode', () => {
  const controller = new DashboardFilterController(() => {}, () => 'mutation-id')
  const current = state(7)
  current.draftControls.state = {
    kind: 'set',
    operator: 'in',
    values: [{ kind: 'string', value: 'OR' }],
  }
  current.dirtyBindings = ['state']
  controller.reconcile(current)

  expect(controller.expression('state')).toEqual(current.draftControls.state)
  expect(controller.projected.appliedControls.state.expression).toEqual(unfiltered)
})

test('filter controller optimistically applies all deferred drafts together', () => {
  const sent: DashboardFilterCommand[] = []
  const controller = new DashboardFilterController(command => sent.push(command), () => 'apply-1')
  controller.setApplicationMode('deferred')
  const current = state(4)
  current.draftControls.state = setExpression('CA')
  current.draftControls.category = setExpression('books')
  current.dirtyBindings = ['category', 'state']
  controller.reconcile(current)

  controller.apply()

  expect(controller.projected.appliedControls.state?.expression).toEqual(setExpression('CA'))
  expect(controller.projected.appliedControls.category?.expression).toEqual(setExpression('books'))
  expect(controller.projected.draftControls).toEqual({})
  expect(controller.projected.dirtyBindings).toEqual([])
  expect(sent[0]?.baseRevision).toBe(4)
})
