package model

type CityFacility struct {
	Id         int    `json:"id" xorm:"id pk autoincr"`
	CityId     int    `json:"cityId" xorm:"cityId"`
	Facilities string `json:"facilities"`
}

func (this *CityFacility) TableName() string {
	return "city_facility"
}
