import React, { useContext, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { fetchStacks, syncStack, applyStack } from '@/clients/stacks/client';
import { reactQueryKeys } from '@/clients/reactQueryConfig';
import { ThemeContext } from '@/contexts/ThemeContext';
import Button from '@/components/core/Button';
import Input from '@/components/core/Input';
import SearchIcon from '@/assets/icons/SearchIcon';

const Stacks: React.FC = () => {
  const { theme } = useContext(ThemeContext);
  const [search, setSearch] = useState('');
  const stacksQuery = useQuery({
    queryKey: reactQueryKeys.stacks,
    queryFn: fetchStacks
  });

  const filteredStacks = useMemo(() => {
    if (!stacksQuery.data?.results) return [];
    return stacksQuery.data.results.filter(
      (stack) =>
        stack.name.toLowerCase().includes(search.toLowerCase()) ||
        stack.path.toLowerCase().includes(search.toLowerCase())
    );
  }, [stacksQuery.data?.results, search]);

  return (
    <div
      className={`flex flex-col flex-1 h-screen min-w-0 p-6 gap-6 ${
        theme === 'light' ? 'bg-primary-100' : 'bg-nuances-black'
      }`}
    >
      <div className="flex items-center justify-between">
        <h1
          className={`text-[32px] font-extrabold ${
            theme === 'light' ? 'text-nuances-black' : 'text-nuances-50'
          }`}
        >
          Stacks
        </h1>
        <Button
          variant={theme === 'light' ? 'primary' : 'secondary'}
          isLoading={stacksQuery.isRefetching}
          onClick={() => stacksQuery.refetch()}
        >
          Refresh
        </Button>
      </div>

      <Input
        variant={theme}
        className="w-full"
        placeholder="Search into stacks"
        leftIcon={<SearchIcon />}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 overflow-auto pb-6">
        {filteredStacks.map((stack) => (
          <div
            key={stack.uid}
            className={`rounded-3xl border p-5 ${
              theme === 'light'
                ? 'bg-white border-primary-200'
                : 'bg-nuances-900 border-nuances-700'
            }`}
          >
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <h2
                  className={`text-xl font-bold truncate ${
                    theme === 'light'
                      ? 'text-nuances-black'
                      : 'text-nuances-50'
                  }`}
                >
                  {stack.name}
                </h2>
                <p
                  className={`text-sm ${
                    theme === 'light'
                      ? 'text-nuances-700'
                      : 'text-nuances-300'
                  }`}
                >
                  {stack.repository}
                </p>
                <p
                  className={`text-sm ${
                    theme === 'light'
                      ? 'text-nuances-700'
                      : 'text-nuances-300'
                  }`}
                >
                  {stack.path}
                </p>
              </div>
              <div className="flex gap-2 shrink-0">
                <Button
                  theme={theme}
                  variant="secondary"
                  onClick={async () => {
                    await syncStack(stack.namespace, stack.name);
                    await stacksQuery.refetch();
                  }}
                >
                  Sync
                </Button>
                <Button
                  variant={theme === 'light' ? 'primary' : 'secondary'}
                  onClick={async () => {
                    await applyStack(stack.namespace, stack.name);
                    await stacksQuery.refetch();
                  }}
                >
                  Apply
                </Button>
              </div>
            </div>

            <div
              className={`mt-4 text-sm ${
                theme === 'light' ? 'text-nuances-700' : 'text-nuances-300'
              }`}
            >
              <p>Branch: {stack.branch}</p>
              <p>Runs: {stack.runCount}</p>
              <p>State: {stack.state}</p>
              <p>Last result: {stack.lastResult || 'No result yet'}</p>
            </div>

            <div className="mt-4 space-y-3">
              {stack.units.map((unit) => (
                <details
                  key={unit.id}
                  className={`rounded-2xl border p-3 ${
                    theme === 'light'
                      ? 'border-primary-200 bg-primary-50'
                      : 'border-nuances-700 bg-nuances-950'
                  }`}
                >
                  <summary
                    className={`cursor-pointer font-semibold ${
                      theme === 'light'
                        ? 'text-nuances-black'
                        : 'text-nuances-50'
                    }`}
                  >
                    {unit.path}
                  </summary>
                  <div
                    className={`mt-3 text-sm space-y-1 ${
                      theme === 'light'
                        ? 'text-nuances-700'
                        : 'text-nuances-300'
                    }`}
                  >
                    <p>State: {unit.state}</p>
                    <p>Last action: {unit.lastAction || 'n/a'}</p>
                    <p>Last plan revision: {unit.lastPlannedRevision || 'n/a'}</p>
                    <p>
                      Last apply revision: {unit.lastAppliedRevision || 'n/a'}
                    </p>
                    <p>Last result: {unit.lastResult || 'No result yet'}</p>
                  </div>
                </details>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default Stacks;
