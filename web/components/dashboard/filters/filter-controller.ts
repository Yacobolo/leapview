import type {
  DashboardAppliedFilterState,
  DashboardFilterCommand,
  DashboardFilterExpression,
  DashboardFilterState,
} from '../../../generated/signals'

type CommandSink = (command: DashboardFilterCommand) => void
type MutationIDFactory = () => string

type WithoutBaseRevision<T> = T extends { baseRevision: number } ? Omit<T, 'baseRevision'> : never
type PendingCommand = WithoutBaseRevision<DashboardFilterCommand>
type MutateCommand = Extract<DashboardFilterCommand | PendingCommand, { kind: 'mutate' }>

const emptyState: DashboardFilterState = {
  revision: 0,
  appliedControls: {},
  draftControls: {},
  dirtyBindings: [],
  defaultsRevision: '',
}

export class DashboardFilterController {
  private canonical: DashboardFilterState = cloneState(emptyState)
  private optimistic: DashboardFilterState = cloneState(emptyState)
  private queue: PendingCommand[] = []
  private inFlight: DashboardFilterCommand | null = null
  private mode: 'immediate' | 'deferred' = 'immediate'

  constructor(
    private readonly send: CommandSink,
    private readonly mutationID: MutationIDFactory = () => crypto.randomUUID(),
  ) {}

  setApplicationMode(mode: 'immediate' | 'deferred') {
    this.mode = mode
  }

  reconcile(state: DashboardFilterState) {
    this.canonical = cloneState(state)
    this.optimistic = cloneState(state)
    this.inFlight = null
    this.projectQueued()
    this.flush()
  }

  get projected(): DashboardFilterState {
    return cloneState(this.optimistic)
  }

  get pending(): boolean {
    return this.inFlight !== null || this.queue.length > 0
  }

  expression(bindingKey: string): DashboardFilterExpression {
    return cloneExpression(
      this.optimistic.draftControls[bindingKey]
      ?? this.optimistic.appliedControls[bindingKey]?.expression
      ?? { kind: 'unfiltered' },
    )
  }

  mutate(bindingKey: string, expression: DashboardFilterExpression) {
    this.enqueue({
      kind: 'mutate',
      clientMutationID: this.mutationID(),
      bindingKey,
      operation: 'set',
      expression: cloneExpression(expression),
    })
  }

  clear(bindingKey: string) {
    this.enqueue({
      kind: 'mutate',
      clientMutationID: this.mutationID(),
      bindingKey,
      operation: 'clear',
    })
  }

  resetBinding(bindingKey: string) {
    this.enqueue({
      kind: 'mutate',
      clientMutationID: this.mutationID(),
      bindingKey,
      operation: 'reset_binding',
    })
  }

  apply() {
    this.enqueue({ kind: 'apply', clientMutationID: this.mutationID() })
  }

  cancel() {
    this.enqueue({ kind: 'cancel', clientMutationID: this.mutationID() })
  }

  reset(scope: 'page' | 'dashboard', bindingKeys: string[]) {
    this.enqueue({
      kind: 'reset',
      clientMutationID: this.mutationID(),
      resetScope: scope,
      bindingKeys: [...bindingKeys],
    })
  }

  private enqueue(command: PendingCommand) {
    this.queue.push(command)
    this.projectCommand(command)
    this.flush()
  }

  private flush() {
    if (this.inFlight || this.queue.length === 0) return
    const pending = this.queue.shift()
    if (!pending) return
    const command = { ...pending, baseRevision: this.canonical.revision } as DashboardFilterCommand
    this.inFlight = command
    this.send(command)
  }

  private projectQueued() {
    if (this.inFlight) this.projectCommand(this.inFlight)
    for (const command of this.queue) this.projectCommand(command)
  }

  private projectCommand(command: PendingCommand | DashboardFilterCommand) {
    if (command.kind === 'mutate' && command.bindingKey) {
      const expression = optimisticExpression(command, this.optimistic, command.bindingKey)
      if (this.mode === 'deferred') {
        this.optimistic.draftControls[command.bindingKey] = expression
        if (!this.optimistic.dirtyBindings.includes(command.bindingKey)) {
          this.optimistic.dirtyBindings = [...this.optimistic.dirtyBindings, command.bindingKey].sort()
        }
        return
      }
      const current = this.optimistic.appliedControls[command.bindingKey]
      this.optimistic.appliedControls[command.bindingKey] = optimisticApplied(current, expression)
      return
    }
    if (command.kind === 'cancel') {
      this.optimistic.draftControls = {}
      this.optimistic.dirtyBindings = []
      return
    }
    if (command.kind === 'apply') {
      for (const bindingKey of this.optimistic.dirtyBindings) {
        const expression = this.optimistic.draftControls[bindingKey]
        if (!expression) continue
        this.optimistic.appliedControls[bindingKey] = optimisticApplied(
          this.optimistic.appliedControls[bindingKey],
          expression,
        )
      }
      this.optimistic.draftControls = {}
      this.optimistic.dirtyBindings = []
    }
  }
}

function optimisticExpression(
  command: MutateCommand,
  state: DashboardFilterState,
  bindingKey: string,
): DashboardFilterExpression {
  switch (command.operation) {
    case 'set':
      return cloneExpression(command.expression ?? { kind: 'unfiltered' })
    case 'clear':
      return { kind: 'unfiltered' }
    case 'reset_binding':
      return cloneExpression(
        state.appliedControls[bindingKey]?.expression ?? { kind: 'unfiltered' },
      )
    default:
      return { kind: 'unfiltered' }
  }
}

function optimisticApplied(
  current: DashboardAppliedFilterState | undefined,
  expression: DashboardFilterExpression,
): DashboardAppliedFilterState {
  return {
    expression: cloneExpression(expression),
    resolvedExpression: cloneExpression(expression),
    evaluatedAt: current?.evaluatedAt,
  }
}

function cloneState(state: DashboardFilterState): DashboardFilterState {
  return {
    revision: state.revision,
    defaultsRevision: state.defaultsRevision,
    dirtyBindings: [...(state.dirtyBindings ?? [])],
    appliedControls: Object.fromEntries(
      Object.entries(state.appliedControls ?? {}).map(([key, applied]) => [key, {
        expression: cloneExpression(applied.expression),
        resolvedExpression: cloneExpression(applied.resolvedExpression),
        ...(applied.evaluatedAt ? { evaluatedAt: applied.evaluatedAt } : {}),
      }]),
    ),
    draftControls: Object.fromEntries(
      Object.entries(state.draftControls ?? {}).map(([key, expression]) => [key, cloneExpression(expression)]),
    ),
  }
}

function cloneExpression(expression: DashboardFilterExpression): DashboardFilterExpression {
  return JSON.parse(JSON.stringify(expression)) as DashboardFilterExpression
}
