import React, { useCallback, useEffect, useState } from 'react';
import Card from '../../ui/Card';
import { GameLab, gameControlPost, Teacher } from './gamecontrolApi';

interface LabsAdminProps {
    labs: GameLab[];
    onChanged: () => void;
}

interface LabDraft {
    id: string;
    name: string;
    range_start: string;
    range_end: string;
}

const emptyLab: LabDraft = { id: '', name: '', range_start: '', range_end: '' };

const LabsAdmin: React.FC<LabsAdminProps> = ({ labs, onChanged }) => {
    const [drafts, setDrafts] = useState<Record<string, LabDraft>>({});
    const [newLab, setNewLab] = useState<LabDraft>(emptyLab);
    const [teachers, setTeachers] = useState<Teacher[]>([]);
    const [newTeacher, setNewTeacher] = useState({ login: '', password: '', lab_id: '' });
    const [passwords, setPasswords] = useState<Record<string, string>>({});
    const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

    const loadTeachers = useCallback(async () => {
        try {
            const res = await fetch('/control/gamecontrol/teachers');
            if (res.ok) {
                setTeachers(await res.json());
            }
        } catch (err) {
            console.error('Error fetching teachers:', err);
        }
    }, []);

    useEffect(() => {
        loadTeachers();
    }, [loadTeachers]);

    useEffect(() => {
        setDrafts({});
    }, [labs]);

    const run = async (path: string, body: object, okText: string, after?: () => void) => {
        const res = await gameControlPost(path, body);
        setMessage({ ok: res.ok, text: res.ok ? okText : res.error || 'Error' });
        if (res.ok) {
            after?.();
            onChanged();
            loadTeachers();
        }
    };

    const draftOf = (lab: GameLab): LabDraft => drafts[lab.id] || lab;

    const setDraft = (lab: GameLab, patch: Partial<LabDraft>) =>
        setDrafts({ ...drafts, [lab.id]: { ...draftOf(lab), ...patch } });

    const teacherOf = (labId: string) => teachers.find((t) => t.lab_id === labId);

    const labName = (labId: string) => labs.find((l) => l.id === labId)?.name || '(laboratorio eliminado)';

    const labsWithoutTeacher = labs.filter((l) => !teacherOf(l.id));

    const labFields = (d: LabDraft, set: (patch: Partial<LabDraft>) => void) => (
        <>
            <input
                className="form-control mr-2 mb-2"
                style={{ width: 200 }}
                placeholder="Nombre (Lab 1)"
                value={d.name}
                onChange={(e) => set({ name: e.target.value })}
            />
            <input
                className="form-control mr-2 mb-2"
                style={{ width: 160 }}
                placeholder="IP inicio"
                value={d.range_start}
                onChange={(e) => set({ range_start: e.target.value })}
            />
            <input
                className="form-control mr-2 mb-2"
                style={{ width: 160 }}
                placeholder="IP fin"
                value={d.range_end}
                onChange={(e) => set({ range_end: e.target.value })}
            />
        </>
    );

    return (
        <>
            {message && (
                <div className={`alert ${message.ok ? 'alert-success' : 'alert-danger'}`}>{message.text}</div>
            )}

            <Card title="Laboratorios">
                <div className="p-3">
                    <p className="text-muted">
                        Cada laboratorio es un rango de IPs. Los rangos no pueden superponerse.
                    </p>
                    {labs.map((lab) => {
                        const d = draftOf(lab);

                        return (
                            <div key={lab.id} className="d-flex flex-wrap align-items-start">
                                {labFields(d, (patch) => setDraft(lab, patch))}
                                <button
                                    className="btn btn-primary mr-2 mb-2"
                                    onClick={() => run('labs/save', d, 'Laboratorio guardado.')}>
                                    Guardar
                                </button>
                                <button
                                    className="btn btn-outline-danger mb-2"
                                    onClick={() => {
                                        if (window.confirm(`¿Eliminar el laboratorio "${lab.name}"?`)) {
                                            run('labs/delete', { id: lab.id }, 'Laboratorio eliminado.');
                                        }
                                    }}>
                                    Eliminar
                                </button>
                            </div>
                        );
                    })}

                    <h6 className="font-weight-bold mt-4">Agregar laboratorio</h6>
                    <div className="d-flex flex-wrap align-items-start">
                        {labFields(newLab, (patch) => setNewLab({ ...newLab, ...patch }))}
                        <button
                            className="btn btn-success mb-2"
                            onClick={() => run('labs/save', newLab, 'Laboratorio agregado.', () => setNewLab(emptyLab))}>
                            Agregar
                        </button>
                    </div>
                </div>
            </Card>

            <Card title="Profesores">
                <div className="p-3">
                    <p className="text-muted">
                        Un profesor por laboratorio. Solo ve y controla su laboratorio, sin acceso al resto del panel.
                    </p>

                    {teachers.map((t) => (
                        <div key={t.login} className="d-flex flex-wrap align-items-center mb-2">
                            <span className="font-weight-bold mr-3" style={{ minWidth: 140 }}>
                                {t.login}
                            </span>
                            <span className="mr-3" style={{ minWidth: 160 }}>
                                {labName(t.lab_id)}
                            </span>
                            <input
                                type="password"
                                className="form-control mr-2"
                                style={{ width: 200 }}
                                placeholder="Nueva contraseña"
                                value={passwords[t.login] || ''}
                                onChange={(e) => setPasswords({ ...passwords, [t.login]: e.target.value })}
                            />
                            <button
                                className="btn btn-primary btn-sm mr-2"
                                disabled={!passwords[t.login]}
                                onClick={() =>
                                    run(
                                        'teachers/save',
                                        { login: t.login, password: passwords[t.login], lab_id: t.lab_id },
                                        'Contraseña actualizada.',
                                        () => setPasswords({ ...passwords, [t.login]: '' }),
                                    )
                                }>
                                Cambiar contraseña
                            </button>
                            <button
                                className="btn btn-outline-danger btn-sm"
                                onClick={() => {
                                    if (window.confirm(`¿Eliminar al profesor "${t.login}"?`)) {
                                        run('teachers/delete', { login: t.login }, 'Profesor eliminado.');
                                    }
                                }}>
                                Eliminar
                            </button>
                        </div>
                    ))}

                    {teachers.length === 0 && <div className="text-muted mb-2">Aún no hay profesores.</div>}

                    <h6 className="font-weight-bold mt-4">Agregar profesor</h6>
                    {labsWithoutTeacher.length === 0 ? (
                        <div className="text-muted">Todos los laboratorios ya tienen un profesor asignado.</div>
                    ) : (
                        <div className="d-flex flex-wrap align-items-start">
                            <input
                                className="form-control mr-2 mb-2"
                                style={{ width: 180 }}
                                placeholder="Usuario"
                                value={newTeacher.login}
                                onChange={(e) => setNewTeacher({ ...newTeacher, login: e.target.value })}
                            />
                            <input
                                type="password"
                                className="form-control mr-2 mb-2"
                                style={{ width: 180 }}
                                placeholder="Contraseña (mín. 6)"
                                value={newTeacher.password}
                                onChange={(e) => setNewTeacher({ ...newTeacher, password: e.target.value })}
                            />
                            <select
                                className="form-control mr-2 mb-2"
                                style={{ width: 200 }}
                                value={newTeacher.lab_id}
                                onChange={(e) => setNewTeacher({ ...newTeacher, lab_id: e.target.value })}>
                                <option value="">Laboratorio...</option>
                                {labsWithoutTeacher.map((l) => (
                                    <option key={l.id} value={l.id}>
                                        {l.name}
                                    </option>
                                ))}
                            </select>
                            <button
                                className="btn btn-success mb-2"
                                onClick={() =>
                                    run('teachers/save', newTeacher, 'Profesor creado.', () =>
                                        setNewTeacher({ login: '', password: '', lab_id: '' }),
                                    )
                                }>
                                Crear profesor
                            </button>
                        </div>
                    )}
                </div>
            </Card>
        </>
    );
};

export default LabsAdmin;
