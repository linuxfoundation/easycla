
import { Injectable } from '@angular/core';
import { AuthService } from './auth.service';

@Injectable({
  providedIn: 'root'
})
export class LfxHeaderService {

  constructor(private auth: AuthService) {
    this.setUserInLFxHeader();
  }

  setUserInLFxHeader(): void {
    setTimeout(() => {
      const lfHeaderEl: any = document.getElementById('lfx-header');
      if (!lfHeaderEl) {
        return;
      }
      this.auth.userProfile$.subscribe((data) => {
        console.log(data);
        if (data) {
          lfHeaderEl.authuser = data;
        }
      });
    }, 2000);
  }
}
