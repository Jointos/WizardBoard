import Phaser from 'phaser';
import './style.css';
import { BoardScene, CANVAS_SIZE, RENDER_SCALE } from './board';
import { App } from './app';

const app = new App();
const board = new BoardScene((sq, x, y) => app.onSquareClick(sq, x, y));

new Phaser.Game({
  type: Phaser.AUTO,
  parent: 'stage',
  width: CANVAS_SIZE * RENDER_SCALE,
  height: CANVAS_SIZE * RENDER_SCALE,
  backgroundColor: '#0b0e14',
  scene: [board],
  scale: {
    mode: Phaser.Scale.FIT,
    autoCenter: Phaser.Scale.CENTER_BOTH,
  },
});

app.attachBoard(board);
void app.start();
