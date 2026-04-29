import { useState } from 'react'

function parseBatchImportLines(raw, t) {
    const lines = raw.split(/\r?\n/).map(line => line.trim()).filter(Boolean)
    const accounts = []
    const apiKeys = []
    const identifiers = []
    const errors = []

    lines.forEach((line, index) => {
        let item
        try {
            item = JSON.parse(line)
        } catch (_err) {
            errors.push(t('accountManager.batchImportLineInvalidJson', { line: index + 1 }))
            return
        }
        if (!item || typeof item !== 'object' || Array.isArray(item)) {
            errors.push(t('accountManager.batchImportLineInvalidObject', { line: index + 1 }))
            return
        }

        const email = String(item.email || '').trim()
        const mobile = String(item.mobile || '').trim()
        const password = String(item.password || '').trim()
        const identifier = email || mobile
        if (!identifier || !password) {
            errors.push(t('accountManager.batchImportLineMissingAccount', { line: index + 1 }))
            return
        }

        accounts.push({
            name: String(item.name || item.api_key_name || '').trim(),
            remark: String(item.remark || '').trim(),
            email,
            mobile,
            password,
            device_id: String(item.device_id || '').trim(),
            proxy_id: String(item.proxy_id || '').trim(),
            total_flash_limit: Number(item.total_flash_limit || 0),
            total_pro_limit: Number(item.total_pro_limit || 0),
        })
        identifiers.push(identifier)

        const key = String(item.api_key || item.key || '').trim()
        if (key) {
            apiKeys.push({
                key,
                name: String(item.api_key_name || item.key_name || item.name || '').trim(),
                remark: String(item.api_key_remark || item.key_remark || '').trim(),
            })
        }
    })

    return { accounts, apiKeys, identifiers, errors }
}

export function useBatchAccountImport({
    apiFetch,
    t,
    onMessage,
    onRefresh,
    fetchAccounts,
    loading,
    testingAll,
    setLoading,
    setTestingAll,
    setBatchProgress,
    setSessionCounts,
    onOpen,
}) {
    const [showBatchImport, setShowBatchImport] = useState(false)
    const [batchImportText, setBatchImportText] = useState('')

    const openBatchImport = () => {
        onOpen?.()
        setBatchImportText('')
        setShowBatchImport(true)
    }

    const closeBatchImport = () => {
        if (loading || testingAll) return
        setShowBatchImport(false)
        setBatchImportText('')
    }

    const testImportedAccounts = async (identifiers) => {
        const uniqueIdentifiers = [...new Set(identifiers.filter(Boolean))]
        if (uniqueIdentifiers.length === 0) return

        setLoading(false)
        setTestingAll(true)
        setBatchProgress({ current: 0, total: uniqueIdentifiers.length, results: [] })

        let successCount = 0
        const results = []
        for (let i = 0; i < uniqueIdentifiers.length; i++) {
            const id = uniqueIdentifiers[i]
            try {
                const testRes = await apiFetch('/admin/accounts/test', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ identifier: id }),
                })
                const testData = await testRes.json()
                if (testData.session_count !== undefined) {
                    setSessionCounts(prev => ({ ...prev, [id]: testData.session_count }))
                }
                results.push({ id, success: Boolean(testData.success), message: testData.message, time: testData.response_time })
                if (testData.success) successCount++
            } catch (e) {
                results.push({ id, success: false, message: e.message })
            }
            setBatchProgress({ current: i + 1, total: uniqueIdentifiers.length, results: [...results] })
        }

        onMessage(successCount === uniqueIdentifiers.length ? 'success' : 'error', t('accountManager.batchImportTestCompleted', {
            success: successCount,
            total: uniqueIdentifiers.length,
        }))
        fetchAccounts(1)
        onRefresh()
        setTestingAll(false)
    }

    const batchImportAccounts = async () => {
        if (!batchImportText.trim()) {
            onMessage('error', t('accountManager.batchImportEmpty'))
            return
        }

        const parsed = parseBatchImportLines(batchImportText, t)
        if (parsed.errors.length > 0) {
            onMessage('error', t('accountManager.batchImportParseFailed', {
                count: parsed.errors.length,
                errors: parsed.errors.slice(0, 3).join('; '),
            }))
            return
        }
        if (parsed.accounts.length === 0) {
            onMessage('error', t('accountManager.batchImportEmpty'))
            return
        }

        setLoading(true)
        try {
            const res = await apiFetch('/admin/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    accounts: parsed.accounts,
                    api_keys: parsed.apiKeys,
                }),
            })
            const data = await res.json()
            if (!res.ok) {
                onMessage('error', data.detail || t('messages.importFailed'))
                return
            }

            onMessage('success', t('accountManager.batchImportSuccess', {
                accounts: data.imported_accounts || 0,
                keys: data.imported_keys || 0,
            }))
            setShowBatchImport(false)
            setBatchImportText('')
            fetchAccounts(1)
            onRefresh()
            await testImportedAccounts(parsed.identifiers)
        } catch (_err) {
            setTestingAll(false)
            onMessage('error', t('messages.networkError'))
        } finally {
            setLoading(false)
        }
    }

    return {
        showBatchImport,
        openBatchImport,
        closeBatchImport,
        batchImportText,
        setBatchImportText,
        batchImportAccounts,
    }
}
