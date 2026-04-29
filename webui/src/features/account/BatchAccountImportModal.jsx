import { Upload, X } from 'lucide-react'

export default function BatchAccountImportModal({
    show,
    t,
    value,
    setValue,
    loading,
    onClose,
    onSubmit,
}) {
    if (!show) return null

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
            <div className="bg-card border border-border rounded-xl shadow-xl w-full max-w-3xl overflow-hidden">
                <div className="p-5 border-b border-border flex items-start justify-between gap-4">
                    <div>
                        <h3 className="font-semibold flex items-center gap-2">
                            <Upload className="w-4 h-4 text-primary" />
                            {t('accountManager.batchImportTitle')}
                        </h3>
                        <p className="text-xs text-muted-foreground mt-1">{t('accountManager.batchImportDesc')}</p>
                    </div>
                    <button
                        onClick={onClose}
                        className="p-1.5 text-muted-foreground hover:text-foreground hover:bg-secondary rounded-md transition-colors"
                        title={t('actions.cancel')}
                    >
                        <X className="w-4 h-4" />
                    </button>
                </div>

                <div className="p-5 space-y-3">
                    <textarea
                        value={value}
                        onChange={e => setValue(e.target.value)}
                        placeholder={t('accountManager.batchImportPlaceholder')}
                        className="w-full min-h-[320px] p-3 font-mono text-xs bg-[#09090b] text-foreground border border-border rounded-lg resize-y focus:outline-none focus:ring-1 focus:ring-ring custom-scrollbar"
                        spellCheck={false}
                    />
                    <p className="text-xs text-muted-foreground">{t('accountManager.batchImportHelp')}</p>
                </div>

                <div className="p-5 border-t border-border flex justify-end gap-2">
                    <button
                        onClick={onClose}
                        disabled={loading}
                        className="px-4 py-2 bg-secondary text-secondary-foreground rounded-lg hover:bg-secondary/80 transition-colors text-sm disabled:opacity-50"
                    >
                        {t('actions.cancel')}
                    </button>
                    <button
                        onClick={onSubmit}
                        disabled={loading}
                        className="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors text-sm font-medium disabled:opacity-50"
                    >
                        {loading ? t('accountManager.batchImportRunning') : t('accountManager.batchImportAction')}
                    </button>
                </div>
            </div>
        </div>
    )
}
