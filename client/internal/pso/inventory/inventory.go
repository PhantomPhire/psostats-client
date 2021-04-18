package inventory

import (
	"errors"
	"fmt"
	"log"

	"github.com/TheTitanrain/w32"
	"github.com/phelix-/psostats/v2/client/internal/numbers"
)

//goland:noinspection GoUnusedConst
const (
	itemArray          = 0x00A8D81C
	itemArrayCount     = 0x00A8D820
	itemId             = 0xD8
	itemOwnerOffset    = 0xE4
	itemCode           = 0xF2
	itemEquippedOffset = 0x190
	itemTypeOffset     = 0xF2
	itemGroupOffset    = 0xF3
	itemKills          = 0xE8
	itemWepGrind       = 0x1F5
	itemWepSpecial     = 0x1F6
	itemWepStats       = 0x1C8
	itemArmSlots       = 0x1B8
	itemFrameDfp       = 0x1B9
	itemFrameEvp       = 0x1BA
	itemBarrierDfp     = 0x1E4
	itemBarrierEvp     = 0x1E5
	itemUnitMod        = 0x1DC
	itemMagStats       = 0x1C0
	itemMagPBHas       = 0x1C8
	itemMagPB          = 0x1C9
	itemMagColor       = 0x1CA
	itemMagSync        = 0x1BE
	itemMagIQ          = 0x1BC
	itemMagTimer       = 0x1B4
	itemToolCount      = 0x104
	itemTechType       = 0x108
	itemMesetaAmount   = 0x100
)

func ReadInventory(handle w32.HANDLE, playerIndex uint8) (map[string]string, error) {
	equipment := make(map[string]string)
	buf, _, ok := w32.ReadProcessMemory(handle, uintptr(itemArrayCount), 2)
	if !ok {
		return equipment, errors.New("could not read item count")
	}
	count := numbers.Uint32From16(buf[0:2])
	buf, _, ok = w32.ReadProcessMemory(handle, uintptr(itemArray), 4)
	if !ok {
		return equipment, errors.New("could not read item array")
	}
	address := numbers.Uint32From16(buf[0:2])
	if count != 0 && address != 0 {
		buf, _, ok = w32.ReadProcessMemory(handle, uintptr(address), uintptr(4*count))
		if !ok {
			return equipment, errors.New("could not read item array")
		}

		for i := 0; i < int(count); i++ {
			itemAddr := numbers.Uint32From16(buf[i*2 : (i*2)+2])
			if itemAddr != 0 {
				itemBuffer, _, ok := w32.ReadProcessMemory(handle, uintptr(itemAddr+0xD8), 4)
				if !ok {
					return equipment, errors.New("could not read item")
				}
				itemId := fmt.Sprintf("%04x%04x", itemBuffer[1], itemBuffer[0])
				// itemDataBuffer, _, ok := w32.ReadProcessMemory(handle, uintptr(itemAddr+0xF2), 8)
				// itemType := itemDataBuffer[0] & 0xFF00
				itemType := readU8(handle, uintptr(itemAddr+itemTypeOffset))
				itemGroup := readU8(handle, uintptr(itemAddr+itemGroupOffset))
				equipped := readU8(handle, uintptr(itemAddr+itemEquippedOffset))&0x01 == 1
				itemOwner := readU8(handle, uintptr(itemAddr+itemOwnerOffset))
				if itemOwner == playerIndex && equipped {
					switch itemType {
					case 0:
						weapon := readWeapon(handle, int(itemAddr), itemId, itemGroup)
						equipment[weapon.Id] = weapon.String()
					case 1:
						switch itemGroup {
						case 1:
							frame := readFrame(handle, int(itemAddr), itemId, itemGroup)
							equipment[frame.Id] = frame.Name
						case 2:
							barrier := readBarrier(handle, int(itemAddr), itemId, itemGroup)
							equipment[barrier.Id] = barrier.Name
						case 3:
							unit := readUnit(handle, int(itemAddr), itemId)
							equipment[unit.Id] = unit.Name
						}
					}
				}
				// log.Printf("%v equipped=%v", itemId, equipped)
				// log.Printf("%04x%04x %04x%04x", itemDataBuffer[1], itemDataBuffer[0], itemDataBuffer[3], itemDataBuffer[2])

			}
		}
	}

	// buf, _, ok := w32.ReadProcessMemory(handle, uintptr(playerAddress+base), uintptr((max-base)+4))
	return equipment, nil
}

func readU8(handle w32.HANDLE, address uintptr) uint8 {
	buf, _, ok := w32.ReadProcessMemory(handle, address, 1)
	if !ok {
		log.Fatalf("Error reading 0x%08x", address)
	}
	return uint8(buf[0])
}

func readU32(handle w32.HANDLE, address uintptr) uint32 {
	buf, _, ok := w32.ReadProcessMemory(handle, address, 4)
	if !ok {
		log.Fatalf("Error reading 0x%08x", address)
	}
	return numbers.Uint32From16(buf[0:2])
}

func getWeaponIndex(handle w32.HANDLE, group uint8, index uint8, typeOffset uint8, sizeSomething uint32) uint32 {
	weaponIndex := uint32(0)
	pmtAddress := readU32(handle, 0x00a8dc94)
	weaponAddress := readU32(handle, uintptr(pmtAddress+uint32(typeOffset)))
	if weaponAddress != 0 {
		groupAddress := weaponAddress + (uint32(group) * 8)
		itemAddress := readU32(handle, uintptr(groupAddress+4)) + (sizeSomething * uint32(index))
		weaponIndex = readU32(handle, uintptr(itemAddress))
	}
	return weaponIndex
}

func readItemName(handle w32.HANDLE, index int) string {
	unitxtPointer := readU32(handle, 0x00a9cd50)
	if unitxtPointer == 0 {
		return "?"
	}
	weaponIndex := readU32(handle, uintptr(unitxtPointer+4))
	weaponNameAddress := readU32(handle, uintptr(weaponIndex+uint32(4*index)))

	weaponName, err := numbers.ReadNullTerminatedString(handle, uintptr(weaponNameAddress))
	if err != nil {
		log.Fatalf("Error getting weapon name %v", err)
	}
	return fmt.Sprintf("%v", weaponName)
}

func readWeapon(handle w32.HANDLE, itemAddr int, itemId string, itemGroup uint8) Weapon {
	itemIndex := readU8(handle, uintptr(itemAddr+0xF4))
	weaponIndex := getWeaponIndex(handle, itemGroup, itemIndex, 0x00, 44)
	weapon := Weapon{
		Id: itemId,
	}
	weapon.Name = readItemName(handle, int(weaponIndex))
	weapon.Grind = readU8(handle, uintptr(itemAddr+itemWepGrind))
	weapon.Special = readU8(handle, uintptr(itemAddr+itemWepSpecial))
	weapon.SpecialName = getWeaponSpecial(weapon.Special)
	// log.Printf("i=%2v itemAddr=%08x - itemId= type=%x grind=%v special=%x",
	// 	i, itemAddr, , itemType, grind, special)
	for j := 0; j < 6; j += 2 {
		area := readU8(handle, uintptr(itemAddr+itemWepStats+j))
		percent := readU8(handle, uintptr(itemAddr+itemWepStats+j+1))
		switch area {
		case 1:
			weapon.Native = percent
		case 2:
			weapon.ABeast = percent
		case 3:
			weapon.Machine = percent
		case 4:
			weapon.Dark = percent
		case 5:
			weapon.Hit = percent
		}
	}
	return weapon
}

type Weapon struct {
	Id          string
	Name        string
	Grind       uint8
	Special     uint8
	SpecialName string
	Native      uint8
	ABeast      uint8
	Machine     uint8
	Dark        uint8
	Hit         uint8
}

func (w *Weapon) String() string {
	grindString := ""
	if w.Grind > 0 {
		grindString = fmt.Sprintf(" +%v", w.Grind)
	}
	specialString := ""
	if len(w.SpecialName) > 0 {
		specialString = fmt.Sprintf(" [%v]", w.SpecialName)
	}
	return fmt.Sprintf("%v%v%v [%v/%v/%v/%v|%v]", w.Name, grindString, specialString, w.Native, w.ABeast, w.Machine, w.Dark, w.Hit)
}

func readFrame(handle w32.HANDLE, itemAddr int, itemId string, itemGroup uint8) Frame {
	itemIndex := readU8(handle, uintptr(itemAddr+0xF4))
	weaponIndex := getWeaponIndex(handle, itemGroup-1, itemIndex, 0x04, 32)
	weapon := Frame{
		Id:   itemId,
		Name: readItemName(handle, int(weaponIndex)),
	}
	return weapon
}

type Frame struct {
	Id   string
	Name string
}

func readBarrier(handle w32.HANDLE, itemAddr int, itemId string, itemGroup uint8) Frame {
	itemIndex := readU8(handle, uintptr(itemAddr+0xF4))
	weaponIndex := getWeaponIndex(handle, itemGroup-1, itemIndex, 0x04, 32)
	weapon := Frame{
		Id:   itemId,
		Name: readItemName(handle, int(weaponIndex)),
	}
	return weapon
}

func readUnit(handle w32.HANDLE, itemAddr int, itemId string) Frame {
	itemIndex := readU8(handle, uintptr(itemAddr+0xF4))
	weaponIndex := getWeaponIndex(handle, 0, itemIndex, 0x08, 20)
	weapon := Frame{
		Id:   itemId,
		Name: readItemName(handle, int(weaponIndex)),
	}
	return weapon
}

func getWeaponSpecial(specialId uint8) string {
	specialName := "?"
	switch specialId {
	case 0:
		specialName = ""
	case 1:
		specialName = "Draw"
	case 2:
		specialName = "Drain"
	case 3:
		specialName = "Fill"
	case 4:
		specialName = "Gush"
	case 5:
		specialName = "Heart"
	case 6:
		specialName = "Mind"
	case 7:
		specialName = "Soul"
	case 8:
		specialName = "Geist"
	case 9:
		specialName = "Master's"
	case 10:
		specialName = "Lord's"
	case 11:
		specialName = "King's"
	case 12:
		specialName = "Charge"
	case 13:
		specialName = "Spirit"
	case 14:
		specialName = "Berserk"
	case 15:
		specialName = "Ice"
	case 16:
		specialName = "Frost"
	case 17:
		specialName = "Freeze"
	case 18:
		specialName = "Blizzard"
	case 19:
		specialName = "Bind"
	case 20:
		specialName = "Hold"
	case 21:
		specialName = "Seize"
	case 22:
		specialName = "Arrest"
	case 23:
		specialName = "Heat"
	case 24:
		specialName = "Fire"
	case 25:
		specialName = "Flame"
	case 26:
		specialName = "Burning"
	case 27:
		specialName = "Shock"
	case 28:
		specialName = "Thunder"
	case 29:
		specialName = "Storm"
	case 30:
		specialName = "Tempest"
	case 31:
		specialName = "Dim"
	case 32:
		specialName = "Shadow"
	case 33:
		specialName = "Dark"
	case 34:
		specialName = "Hell"
	case 35:
		specialName = "Panic"
	case 36:
		specialName = "Riot"
	case 37:
		specialName = "Havoc"
	case 38:
		specialName = "Chaos"
	case 39:
		specialName = "Devil's"
	case 40:
		specialName = "Demon's"
	}
	return specialName
}
