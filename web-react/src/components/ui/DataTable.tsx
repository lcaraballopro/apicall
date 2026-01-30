import { cn } from '@/lib/utils';

interface Column<T> {
    key: string;
    header: string;
    render?: (item: T) => React.ReactNode;
    className?: string;
}

interface DataTableProps<T> {
    data: T[];
    columns: Column<T>[];
    isLoading?: boolean;
    emptyMessage?: string;
}

export function DataTable<T>({
    data,
    columns,
    isLoading,
    emptyMessage = 'No hay datos',
}: DataTableProps<T>) {
    if (isLoading) {
        return (
            <div className="rounded-xl border border-[hsl(var(--border))] overflow-hidden">
                <div className="p-8 text-center text-gray-400">
                    <div className="animate-spin w-8 h-8 border-2 border-[hsl(var(--primary))] border-t-transparent rounded-full mx-auto mb-2" />
                    Cargando...
                </div>
            </div>
        );
    }

    return (
        <div className="rounded-xl border border-[hsl(var(--border))] overflow-hidden">
            <table className="w-full">
                <thead>
                    <tr className="bg-[hsl(var(--secondary))]">
                        {columns.map((col, i) => (
                            <th
                                key={i}
                                className={cn(
                                    'px-4 py-3 text-left text-sm font-semibold text-gray-300',
                                    col.className
                                )}
                            >
                                {col.header}
                            </th>
                        ))}
                    </tr>
                </thead>
                <tbody>
                    {data.length === 0 ? (
                        <tr>
                            <td colSpan={columns.length} className="px-4 py-8 text-center text-gray-500">
                                {emptyMessage}
                            </td>
                        </tr>
                    ) : (
                        data.map((item, rowIdx) => (
                            <tr
                                key={rowIdx}
                                className="border-t border-[hsl(var(--border))] hover:bg-[hsl(var(--secondary))/30] transition-colors"
                            >
                                {columns.map((col, colIdx) => (
                                    <td key={colIdx} className={cn('px-4 py-3', col.className)}>
                                        {col.render
                                            ? col.render(item)
                                            : String((item as Record<string, unknown>)[col.key] ?? '')}
                                    </td>
                                ))}
                            </tr>
                        ))
                    )}
                </tbody>
            </table>
        </div>
    );
}
