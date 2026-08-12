import React, { useEffect, useState } from 'react';
import PageTitle from '../../ui/PageTitle';
import Card from '../../ui/Card';

interface GameHost {
    ip: string;
    host: string;
    blocked: boolean;
}

interface GameControlStatus {
    enabled: boolean;
    upstream_url: string;
    range_start: string;
    range_end: string;
    hosts: GameHost[];
}

const GameControl: React.FC = () => {
    const [status, setStatus] = useState<GameControlStatus | null>(null);
    const [loading, setLoading] = useState<boolean>(true);
    const [search, setSearch] = useState<string>('');

    const fetchStatus = async () => {
        try {
            setLoading(true);
            const res = await fetch('/control/gamecontrol/status');
            if (res.ok) {
                const data = await res.json();
                setStatus(data);
            }
        } catch (err) {
            console.error('Error fetching GameControl status:', err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchStatus();
    }, []);

    const handleToggleHost = async (ip: string, currentBlocked: boolean) => {
        try {
            const res = await fetch('/control/gamecontrol/update_host', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ip, blocked: !currentBlocked }),
            });
            if (res.ok) {
                fetchStatus();
            }
        } catch (err) {
            console.error('Error updating host:', err);
        }
    };

    const handleToggleAll = async (blocked: boolean) => {
        try {
            const res = await fetch('/control/gamecontrol/toggle_all', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ blocked }),
            });
            if (res.ok) {
                fetchStatus();
            }
        } catch (err) {
            console.error('Error toggling all hosts:', err);
        }
    };

    const filteredHosts = status?.hosts.filter(
        (h) =>
            h.host.toLowerCase().includes(search.toLowerCase()) ||
            h.ip.includes(search),
    ) || [];

    return (
        <div>
            <PageTitle title="GameControl - Control de Juegos por IP" />
            <Card title="Panel de Administración de Juegos">
                {loading && <div>Cargando GameControl...</div>}
                {!loading && status && (
                    <div className="p-3">
                        <div className="d-flex justify-content-between align-items-center mb-4 flex-wrap gap-2">
                            <div>
                                <span className="font-weight-bold mr-2">Estado del Módulo:</span>
                                <span className={`badge ${status.enabled ? 'badge-success' : 'badge-secondary'}`}>
                                    {status.enabled ? 'Activo' : 'Inactivo'}
                                </span>
                            </div>
                            <div className="btn-group">
                                <button
                                    className="btn btn-danger btn-sm"
                                    onClick={() => handleToggleAll(true)}>
                                    Bloquear Todo el Laboratorio
                                </button>
                                <button
                                    className="btn btn-success btn-sm"
                                    onClick={() => handleToggleAll(false)}>
                                    Desbloquear Todo el Laboratorio
                                </button>
                            </div>
                        </div>

                        <div className="mb-3">
                            <input
                                type="text"
                                className="form-control"
                                placeholder="Buscar por Nombre de Equipo (PC1, PC2...) o IP..."
                                value={search}
                                onChange={(e) => setSearch(e.target.value)}
                            />
                        </div>

                        <div className="table-responsive">
                            <table className="table table-vcenter card-table">
                                <thead>
                                    <tr>
                                        <th>Equipo / Host</th>
                                        <th>Dirección IP</th>
                                        <th>Estado de Acceso a Juegos</th>
                                        <th className="text-right">Acción</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {filteredHosts.map((h) => (
                                        <tr key={h.ip}>
                                            <td className="font-weight-bold">{h.host}</td>
                                            <td>{h.ip}</td>
                                            <td>
                                                <span className={`badge ${h.blocked ? 'badge-danger' : 'badge-success'}`}>
                                                    {h.blocked ? 'Bloqueado' : 'Permitido'}
                                                </span>
                                            </td>
                                            <td className="text-right">
                                                <div className="custom-control custom-switch d-inline-block">
                                                    <input
                                                        type="checkbox"
                                                        className="custom-control-input"
                                                        id={`switch-${h.ip}`}
                                                        checked={h.blocked}
                                                        onChange={() => handleToggleHost(h.ip, h.blocked)}
                                                    />
                                                    <label
                                                        className="custom-control-label"
                                                        htmlFor={`switch-${h.ip}`}>
                                                        {h.blocked ? 'Desbloquear' : 'Bloquear'}
                                                    </label>
                                                </div>
                                            </td>
                                        </tr>
                                    ))}
                                    {filteredHosts.length === 0 && (
                                        <tr>
                                            <td colSpan={4} className="text-center text-muted">
                                                No se encontraron equipos en el rango.
                                            </td>
                                        </tr>
                                    )}
                                </tbody>
                            </table>
                        </div>
                    </div>
                )}
            </Card>
        </div>
    );
};

export default GameControl;
