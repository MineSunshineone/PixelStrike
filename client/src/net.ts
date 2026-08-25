import { OP, PROTOCOL_VERSION, type PlayerSnap, type RosterEntry } from './constants.js';

export interface GameEvent { type:number; killer?:number; victim?:number; player?:number; weapon?:number; headshot?:number; origin?:[number,number,number]; dir?:[number,number,number]; name?:string; pickup?:number; kind?:number; ms?:number; streak?:number }
export interface SelfState { ack:number; slot:number; weapon:number; weaponSkin:number; mag:number; reserve:number; nades:number }

export class Net {
  ws!:WebSocket; yourId=0; connected=false; lastServerTick=0;
  onWelcome:(id:number,revision:number)=>void=()=>{};
  onSnapshot:(tick:number,ack:number,players:PlayerSnap[])=>void=()=>{};
  onEvents:((events:GameEvent[])=>void)|null=null;
  onRoster:((rows:RosterEntry[])=>void)|null=null;
  onSelf:((state:SelfState)=>void)|null=null;
  onLatency:((ms:number,outboundBps:number)=>void)|null=null;
  onDisconnect:((upgrading:boolean)=>void)|null=null; onReject:((reason:string)=>void)|null=null; onMaintenance:((retryAfter:number)=>void)|null=null;
  private name=''; private primary=3; private secondary=0; private skin=0; private primaryWeaponSkin=0; private secondaryWeaponSkin=0; private url=''; private retry=0; private closedByUser=false; private heartbeat=0; private lastPing=0; private maintenanceUntil=0; private states=new Map<number,PlayerSnap>(); private input=new Uint8Array(12); private inputView=new DataView(this.input.buffer);

  connect(url:string,name:string,primary:number,secondary:number,skin:number,primaryWeaponSkin:number,secondaryWeaponSkin:number){ this.disconnect();this.url=url;this.name=name;this.primary=primary;this.secondary=secondary;this.skin=skin;this.primaryWeaponSkin=primaryWeaponSkin;this.secondaryWeaponSkin=secondaryWeaponSkin;this.closedByUser=false;this.open();this.heartbeat=window.setInterval(()=>{if(this.connected){this.lastPing=performance.now();this.raw(new Uint8Array([OP.Ping]))}},5000) }
  private open() {
    const ws = new WebSocket(this.url);
    this.ws = ws;
    ws.binaryType = 'arraybuffer';
    ws.onopen = () => {
      if (this.ws !== ws) return;
      this.states.clear();
      const nb = new TextEncoder().encode([...this.name].slice(0, 16).join(''));
      const fp = new TextEncoder().encode(fingerprint());
      const b = new Uint8Array(8 + nb.length + fp.length);
      b[0] = OP.Join;
      b[1] = PROTOCOL_VERSION;
      b[2] = nb.length;
      b.set(nb, 3);
      b[3 + nb.length] = this.primary;
      b[4 + nb.length] = this.secondary;
      b[5 + nb.length] = this.skin;
      b[6 + nb.length] = this.primaryWeaponSkin;
      b[7 + nb.length] = this.secondaryWeaponSkin;
      b.set(fp, 8 + nb.length);
      ws.send(b);
    };
    ws.onmessage = (e) => {
      if (this.ws === ws) this.handle(new DataView(e.data));
    };
    ws.onclose = () => {
      if (this.ws !== ws) return;
      this.connected = false;
      if (this.closedByUser) return;
      const upgrading = this.maintenanceUntil > performance.now();
      this.onDisconnect?.(upgrading);
      const delay = Math.max(Math.min(5000, 500 * 2 ** this.retry++), this.maintenanceUntil - performance.now());
      setTimeout(() => {
        if (!this.closedByUser && this.ws === ws) this.open();
      }, delay);
    };
  }
  private handle(v:DataView) {
    const op = v.getUint8(0);
    if (op === OP.Welcome) {
      if (v.getUint8(1) !== PROTOCOL_VERSION) { this.onReject?.('协议版本不一致'); return; }
      this.yourId = v.getUint16(2, true); this.connected = true; this.retry = 0; this.maintenanceUntil = 0;
      this.onWelcome(this.yourId, v.getUint32(4, true)); return;
    }
    if (op === OP.Maintenance) {
      const retryAfter = v.byteLength >= 2 ? v.getUint8(1) : 2;
      this.maintenanceUntil = performance.now() + retryAfter * 1000;
      this.onMaintenance?.(retryAfter); return;
    }
    if (op === OP.Pong) { if (this.lastPing) this.onLatency?.(performance.now() - this.lastPing, v.byteLength >= 5 ? v.getUint32(1, true) : 0); return; }
    if (op === OP.Reject) { const n=v.getUint8(1),reason=new TextDecoder().decode(new Uint8Array(v.buffer,v.byteOffset+2,n));this.closedByUser=true;this.onReject?.(reason);this.ws.close();return; }
    if (op === OP.Snapshot) { this.decodeSnapshot(v); return; }
    if (op === OP.Events) { this.decodeEvents(v); return; }
    if (op === OP.Self) { this.onSelf?.({ack:v.getUint16(1,true),slot:v.getUint8(3),weapon:v.getUint8(4),weaponSkin:v.getUint8(5),mag:v.getUint8(6),reserve:v.getUint16(7,true),nades:v.getUint8(9)}); return; }
    if (op === OP.Roster) {
      let o=2; const rows:RosterEntry[]=[];
      for(let i=0,n=v.getUint8(1);i<n;i++){const id=v.getUint16(o,true),kills=v.getUint16(o+2,true),deaths=v.getUint16(o+4,true),len=v.getUint8(o+6);o+=7;const name=new TextDecoder().decode(new Uint8Array(v.buffer,v.byteOffset+o,len));o+=len;rows.push({id,name,kills,deaths})}
      this.onRoster?.(rows);
    }
  }
  private decodeSnapshot(v:DataView){const tick=v.getUint32(1,true),ack=v.getUint16(5,true),n=v.getUint8(7);this.lastServerTick=tick;let o=8;const updated:PlayerSnap[]=[];for(let i=0;i<n;i++){const id=v.getUint16(o,true),mask=v.getUint16(o+2,true);o+=4;let s=this.states.get(id);if(mask===0x8000){s={id,x:v.getInt16(o,true)/100,y:v.getInt16(o+2,true)/100,z:v.getInt16(o+4,true)/100,yaw:half(v.getInt16(o+6,true)),pitch:half(v.getInt16(o+8,true)),vx:v.getInt8(o+10)/10,vz:v.getInt8(o+11)/10,hp:v.getUint8(o+12),armor:v.getUint8(o+13),state:v.getUint8(o+14),weapon:v.getUint8(o+15),shot:v.getUint8(o+16),skin:v.getUint8(o+17),weaponSkin:v.getUint8(o+18)};o+=19}else{if(!s){if(mask&1)o+=3;else if(mask&2)o+=6;if(mask&4)o+=2;else if(mask&8)o+=4;if(mask&16)o+=2;if(mask&32)o+=2;if(mask&64)o+=1;if(mask&128)o+=1;if(mask&256)o+=1;if(mask&512)o+=1;if(mask&1024)o+=1;continue}if(mask&1){s.x+=v.getInt8(o++)/100;s.y+=v.getInt8(o++)/100;s.z+=v.getInt8(o++)/100}else if(mask&2){s.x=v.getInt16(o,true)/100;s.y=v.getInt16(o+2,true)/100;s.z=v.getInt16(o+4,true)/100;o+=6}if(mask&4){s.yaw=wrap(s.yaw+half(v.getInt8(o++)));s.pitch+=half(v.getInt8(o++))}else if(mask&8){s.yaw=half(v.getInt16(o,true));s.pitch=half(v.getInt16(o+2,true));o+=4}if(mask&16){s.vx=v.getInt8(o++)/10;s.vz=v.getInt8(o++)/10}if(mask&32){s.hp=v.getUint8(o++);s.armor=v.getUint8(o++)}if(mask&64)s.state=v.getUint8(o++);if(mask&128)s.weapon=v.getUint8(o++);if(mask&256)s.shot=v.getUint8(o++);if(mask&512)s.skin=v.getUint8(o++);if(mask&1024)s.weaponSkin=v.getUint8(o++)}this.states.set(id,s);updated.push(s)}this.onSnapshot(tick,ack,updated)}
  private decodeEvents(v: DataView) {
    const rows: GameEvent[] = [];
    let o = 2;
    for (let i = 0, n = v.getUint8(1); i < n; i++) {
      const e: GameEvent = { type: v.getUint8(o++) };
      switch (e.type) {
        case 0:
          e.killer = v.getUint16(o, true); e.victim = v.getUint16(o + 2, true);
          e.weapon = v.getUint8(o + 4); e.headshot = v.getUint8(o + 5); o += 6; break;
        case 1:
          e.player = v.getUint16(o, true); e.victim = v.getUint16(o + 2, true);
          e.headshot = v.getUint8(o + 4) >> 7; o += 5; break;
        case 2:
          e.player = v.getUint16(o, true); e.origin = vec(v, o + 2); o += 14; break;
        case 5:
          e.player = v.getUint16(o, true); e.ms = v.getUint16(o + 2, true); o += 4; break;
        case 6: {
          e.player = v.getUint16(o, true);
          const len = v.getUint8(o + 2);
          e.name = new TextDecoder().decode(new Uint8Array(v.buffer, v.byteOffset + o + 3, len));
          o += 3 + len; break;
        }
        case 7:
          e.origin = vec(v, o); o += 12; break;
        case 8:
          e.player = v.getUint16(o, true); e.origin = vec(v, o + 2); e.dir = vec(v, o + 14); o += 26; break;
        case 9:
          e.player = v.getUint16(o, true); o += 2; break;
        case 10:
          e.pickup = v.getUint16(o, true); e.kind = v.getUint8(o + 2); e.origin = vec(v, o + 3); o += 15; break;
        case 11:
          e.pickup = v.getUint16(o, true); e.victim = v.getUint16(o + 2, true);
          e.kind = v.getUint8(o + 4); e.ms = v.getUint16(o + 5, true); o += 7; break;
        case 12: {
          e.player = v.getUint16(o, true); e.kind = v.getUint8(o + 2);
          const len = v.getUint8(o + 3);
          e.name = new TextDecoder().decode(new Uint8Array(v.buffer, v.byteOffset + o + 4, len));
          o += 4 + len; break;
        }
        case 13:
          e.player = v.getUint16(o, true); e.kind = v.getUint8(o + 2);
          e.streak = v.getUint8(o + 3); e.ms = v.getUint16(o + 4, true); o += 6; break;
      }
      rows.push(e);
    }
    this.onEvents?.(rows);
  }
  sendInput(seq:number,keys:number,yaw:number,pitch:number){const b=this.input,v=this.inputView;b[0]=OP.Input;v.setUint16(1,seq,true);b[3]=keys;v.setFloat32(4,yaw,true);v.setFloat32(8,pitch,true);this.raw(b)}
  sendFire(seq:number,tick:number,mode:number,yaw:number,pitch:number){const b=new Uint8Array(16),v=new DataView(b.buffer);b[0]=OP.Fire;v.setUint16(1,seq,true);v.setUint32(3,tick,true);b[7]=mode;v.setFloat32(8,yaw,true);v.setFloat32(12,pitch,true);this.raw(b)}
  sendReload(){this.raw(new Uint8Array([OP.Reload]))} switchSlot(slot:number){this.raw(new Uint8Array([OP.Switch,slot]))} setLoadout(primary:number,secondary:number,primaryWeaponSkin:number,secondaryWeaponSkin:number){this.raw(new Uint8Array([OP.Loadout,primary,secondary,primaryWeaponSkin,secondaryWeaponSkin]))} requestRoster(){this.raw(new Uint8Array([OP.RosterRequest]))} toggleFlight(){this.raw(new Uint8Array([OP.ToggleFlight]))}
  sendGrenade(yaw:number,pitch:number){const b=new Uint8Array(9),v=new DataView(b.buffer);b[0]=OP.Grenade;v.setFloat32(1,yaw,true);v.setFloat32(5,pitch,true);this.raw(b)}
  forget(id:number){this.states.delete(id)}
  disconnect(){this.closedByUser=true;if(this.heartbeat){clearInterval(this.heartbeat);this.heartbeat=0}const ws=this.ws;if(ws){ws.onopen=null;ws.onmessage=null;ws.onclose=null;if(ws.readyState<2)ws.close()}this.connected=false;this.states.clear()}
  private raw(b:Uint8Array<ArrayBuffer>){if(this.ws?.readyState===WebSocket.OPEN)this.ws.send(b)}
}
const half=(v:number)=>v*.5*Math.PI/180,wrap=(v:number)=>Math.atan2(Math.sin(v),Math.cos(v)),vec=(v:DataView,o:number):[number,number,number]=>[v.getFloat32(o,true),v.getFloat32(o+4,true),v.getFloat32(o+8,true)];
function fingerprint(){let fp=localStorage.getItem('pixel_strike_fp');if(!fp){fp='fp_'+crypto.getRandomValues(new Uint32Array(2)).join('_');localStorage.setItem('pixel_strike_fp',fp)}return fp}
